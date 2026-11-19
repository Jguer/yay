package scan

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// staticRule is a deterministic, line-oriented heuristic.
type staticRule struct {
	id       string
	severity Severity
	summary  string
	re       *regexp.Regexp
}

func rule(id string, sev Severity, summary, pattern string) staticRule {
	return staticRule{id: id, severity: sev, summary: summary, re: regexp.MustCompile(pattern)}
}

// Rules target build-time and install-time malware seen in real AUR incidents:
// remote execution, obfuscated payloads, persistence, privilege escalation,
// exfiltration and miner behavior. Benign packaging idioms (rm -rf "$pkgdir",
// install -Dm755, desktop files) are deliberately not matched.
var staticRules = []staticRule{
	rule("remote-exec-pipe", SeverityCritical,
		"downloads content and pipes it directly into a shell/interpreter",
		`(?i)\b(curl|wget|fetch)\b[^|;&\n]*\|\s*(sudo\s+)?(ba|z|k|da)?sh\b|\b(curl|wget)\b[^|;&\n]*\|\s*(python|perl|ruby|node|php)\b`),
	rule("remote-exec-eval", SeverityCritical,
		"evaluates downloaded content",
		`(?i)\beval\b[^\n]*\$\((curl|wget)\b|\beval\b[^\n]*"(curl|wget)\b`),
	rule("obfuscated-shell-pipe", SeverityCritical,
		"decodes data and pipes it into a shell",
		`(?i)(base64\s+(-d|--decode|-D)|openssl\s+(enc\s+)?-d|xxd\s+-r)[^|;&\n]*\|\s*(sudo\s+)?(ba|z|k)?sh\b`),
	rule("reverse-shell", SeverityCritical,
		"opens a reverse shell",
		`(?i)/dev/tcp/|\bnc(at)?\b[^\n]*\s-e\s|\bbash\s+-i\s+>&|\bsocat\b[^\n]*exec:`),
	rule("miner-stratum", SeverityCritical,
		"references a mining pool protocol",
		`(?i)stratum\+tcp://|\b(xmrig|minerd|cpuminer)\b`),
	rule("persistence-cron", SeverityCritical,
		"modifies cron configuration",
		`(?i)\bcrontab\b|/etc/cron\.(d|hourly|daily|weekly)/|/var/spool/cron`),
	rule("persistence-autostart", SeverityCritical,
		"writes autostart or shell-rc persistence outside the package root",
		`(?i)(cp|mv|install|ln|tee|mkdir|sed\s+-i|chmod)\b[^\n]*(~|\$HOME|\$\{HOME\}|/home/[^/\s]+)/\.(config/autostart|bashrc|zshrc|profile|bash_profile)|(>>?)\s*(~|\$HOME|\$\{HOME\}|/home/[^/\s]+)/\.(bashrc|zshrc|profile|bash_profile)`),
	rule("persistence-systemd", SeverityWarning,
		"touches systemd units; verify this is packaged, not written to the live system",
		`(?i)(systemctl\s+(enable|start)\b|/etc/systemd/system|~/.config/systemd)`),
	rule("ssh-key-injection", SeverityCritical,
		"writes SSH authorized_keys",
		`authorized_keys`),
	rule("ld-preload", SeverityCritical,
		"injects a library via LD_PRELOAD/LD_LIBRARY_PATH",
		`\bLD_PRELOAD\b|/etc/ld\.so\.preload`),
	rule("setuid", SeverityWarning,
		"sets setuid/setgid bits",
		`(?i)\bchmod\b[^\n]*\b(u\+s|g\+s|[2467][0-7]{3})\b`),
	rule("sudoers-modify", SeverityCritical,
		"modifies sudo configuration",
		`/etc/sudoers|\bvisudo\b`),
	rule("destructive-rm", SeverityCritical,
		"recursively deletes a system or home path",
		`\brm\s+(-[a-zA-Z]*[rf][a-zA-Z]*[rf]?[a-zA-Z]*\s+)+\s*("?)(/|~|\$HOME|\$\{HOME\}|/etc|/usr|/boot|/var)\b`),
	rule("webhook-exfil", SeverityCritical,
		"posts data to a chat webhook (common exfiltration channel)",
		`(?i)discord(app)?\.com/api/webhooks|api\.telegram\.org/bot|hooks\.slack\.com`),
	rule("paste-payload", SeverityWarning,
		"fetches from a paste/file-drop host; verify the payload",
		`(?i)(pastebin\.com/raw|paste\.ee|hastebin\.com|ghostbin|transfer\.sh|0x0\.st|temp\.sh|file\.io)/`),
	rule("ip-literal-url", SeverityWarning,
		"downloads from a bare IP address instead of a domain",
		`https?://[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}([:/]|$)`),
	rule("insecure-http-source", SeverityInfo,
		"uses plaintext HTTP; integrity relies on checksums alone",
		`(?i)\bhttp://`),
	rule("tls-verification-disabled", SeverityWarning,
		"disables TLS certificate verification for a download",
		`(?i)\bcurl\b[^\n|;&]*\s(-k|--insecure)\b|\bwget\b[^\n|;&]*--no-check-certificate\b`),
	rule("checksum-skipped", SeverityInfo,
		"skips source integrity checksums (normal for VCS sources, risky for tarballs)",
		`(?i)(sha256|sha512|sha1|md5|b2)sums[^)]*'SKIP'`),
	rule("background-daemon", SeverityWarning,
		"launches a detached background process",
		`(?i)\b(nohup|setsid|disown)\b|&\s*(>/dev/null|2>&1)`),
	rule("tmp-execution", SeverityWarning,
		"executes or sources from a world-writable temp directory",
		`(?i)(/tmp|/var/tmp|/dev/shm)/[^\s;&|]*\.(sh|py|pl|elf|bin)\b`),
	rule("long-base64-blob", SeverityWarning,
		"contains a long base64 blob (possible embedded payload)",
		`[A-Za-z0-9+/]{240,}={0,2}`),
	rule("anti-analysis", SeverityWarning,
		"contains sandbox/debugger evasion or log tampering",
		`(?i)\b(history\s+-c|unset\s+HISTFILE|shred\s+-u|/dev/null\s+2>&1\s*&\s*$)`),
}

const (
	staticMaxFileSize  = 4 << 20 // 4 MiB per file
	staticMaxFiles     = 2000
	staticMaxFindings  = 200
	staticMaxEvidence  = 160
	staticSnippetBytes = 8192
)

// Static scans every text file below root with the heuristic rule set.
// Binary and oversized files are skipped; the agent layer can still list them.
func Static(root string) ([]Finding, error) {
	findings := []Finding{}
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err // missing/unreadable root must fail the scan
			}
			return nil
		}
		if len(findings) >= staticMaxFindings {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || files >= staticMaxFiles {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.Size() > staticMaxFileSize {
			return nil
		}
		files++
		data, readErr := os.ReadFile(path)
		if readErr != nil || isBinary(data) {
			return nil
		}
		rel := relativePath(root, path)
		findings = append(findings, scanText(rel, data)...)
		return nil
	})
	if err != nil {
		return findings, err
	}
	if len(findings) > staticMaxFindings {
		findings = findings[:staticMaxFindings]
	}
	return findings, nil
}

func scanText(rel string, data []byte) []Finding {
	var out []Finding
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		for _, r := range staticRules {
			if !r.re.MatchString(line) {
				continue
			}
			out = append(out, Finding{
				Source:   "static",
				Severity: r.severity,
				Rule:     r.id,
				File:     rel,
				Line:     i + 1,
				Summary:  r.summary,
				Evidence: truncate(strings.TrimSpace(line), staticMaxEvidence),
			})
			if len(out) >= staticMaxFindings {
				return out
			}
		}
	}
	return out
}

func isBinary(data []byte) bool {
	n := len(data)
	if n > staticSnippetBytes {
		n = staticSnippetBytes
	}
	return bytes.IndexByte(data[:n], 0) >= 0
}

func relativePath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
