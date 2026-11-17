// Package scan implements pre-build malware scanning of AUR packages.
//
// Deterministic heuristics run over a package's build directory: the
// PKGBUILD, install scripts and the downloaded sources, which are unpacked
// with bsdtar (never makepkg, which would execute package code).
package scan

import (
	"strings"
	"time"
)

// Verdict is the outcome of scanning one package.
type Verdict string

const (
	VerdictClean      Verdict = "clean"
	VerdictSuspicious Verdict = "suspicious"
	VerdictMalicious  Verdict = "malicious"
	// VerdictUnknown means the scan could not complete. Policy decides
	// whether that fails open (warn) or closed (--scanstrict).
	VerdictUnknown Verdict = "unknown"
)

// rank orders verdicts from least to most severe; Unknown is least severe so a
// failed scan can never mask a concrete finding.
func (v Verdict) rank() int {
	switch v {
	case VerdictUnknown:
		return 0
	case VerdictClean:
		return 1
	case VerdictSuspicious:
		return 2
	case VerdictMalicious:
		return 3
	}
	return 0
}

// Worst returns the more severe of two verdicts.
func Worst(a, b Verdict) Verdict {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// Severity describes how bad a single finding is.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

func (s Severity) verdict() Verdict {
	switch s {
	case SeverityCritical:
		return VerdictMalicious
	case SeverityWarning:
		return VerdictSuspicious
	}
	return VerdictClean
}

// Finding is one concrete observation about a file.
type Finding struct {
	Source   string   `json:"source"` // "static"
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule,omitempty"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Summary  string   `json:"summary"`
	Evidence string   `json:"evidence,omitempty"`
}

// Report is the full result for one package, persisted as JSON.
type Report struct {
	Package   string    `json:"package"`
	Directory string    `json:"directory"`
	Verdict   Verdict   `json:"verdict"`
	Findings  []Finding `json:"findings,omitempty"`

	// SourcesExtracted counts source archives unpacked for inspection.
	SourcesExtracted int           `json:"sourcesExtracted"`
	Error            string        `json:"error,omitempty"`
	Duration         time.Duration `json:"duration"`
	Started          time.Time     `json:"started"`

	// ContentHash and ScannerVersion key the on-disk report cache.
	ContentHash    string `json:"contentHash,omitempty"`
	ScannerVersion int    `json:"scannerVersion"`
	// Cached is true when this report was loaded instead of rescanned.
	Cached bool `json:"-"`
}

// FindingsVerdict folds findings into a single verdict.
func FindingsVerdict(findings []Finding) Verdict {
	if len(findings) == 0 {
		return VerdictClean
	}
	v := VerdictClean
	for _, f := range findings {
		v = Worst(v, f.Severity.verdict())
	}
	return v
}

// Format renders a human-readable report for the terminal.
func (r *Report) Format() string {
	var b strings.Builder
	b.WriteString("scan: ")
	b.WriteString(r.Package)
	b.WriteString(": ")
	b.WriteString(string(r.Verdict))
	b.WriteByte('\n')
	for _, f := range r.Findings {
		b.WriteString("  [")
		b.WriteString(string(f.Severity))
		b.WriteString("] ")
		if f.File != "" {
			b.WriteString(f.File)
			if f.Line > 0 {
				b.WriteString(":")
				b.WriteString(itoa(f.Line))
			}
			b.WriteString(": ")
		}
		b.WriteString(f.Summary)
		if f.Rule != "" {
			b.WriteString(" (")
			b.WriteString(f.Rule)
			b.WriteByte(')')
		}
		b.WriteByte('\n')
	}
	if r.Error != "" {
		b.WriteString("  error: ")
		b.WriteString(r.Error)
		b.WriteByte('\n')
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
