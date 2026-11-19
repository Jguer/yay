package scan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Options controls one package scan.
type Options struct {
	Package string
	Dir     string // build directory containing PKGBUILD and sources

	ReportDir string // if set, the JSON report is written here
}

// Scanner runs the static heuristics for one package.
type Scanner struct {
	opts Options
}

func New(opts Options) *Scanner { return &Scanner{opts: opts} }

// Run scans and returns a report; the report is always non-nil. A non-nil
// error means the scan is incomplete and the verdict must not be trusted.
func (s *Scanner) Run(ctx context.Context) (*Report, error) {
	start := time.Now()
	report := &Report{
		Package:   s.opts.Package,
		Directory: s.opts.Dir,
		Started:   start,
		Verdict:   VerdictUnknown,
	}

	// Fingerprint the inputs before unpacking anything, for the report cache.
	if hash, hashErr := ContentHash(s.opts.Dir); hashErr == nil {
		report.ContentHash = hash
	}
	report.ScannerVersion = ScannerVersion

	// Unpack downloaded archives without executing any package code.
	// The scratch dir is removed again before the build.
	cleanup, extracted := ExtractSources(ctx, s.opts.Dir)
	defer cleanup()
	report.SourcesExtracted = extracted

	findings, err := Static(s.opts.Dir)
	if err != nil {
		// An unreadable or missing build dir is not a clean package.
		report.Error = err.Error()
		report.Duration = time.Since(start)
		return report, err
	}
	report.Findings = findings
	report.Verdict = FindingsVerdict(findings)
	report.Duration = time.Since(start)

	if s.opts.ReportDir != "" {
		if err := saveReport(s.opts.ReportDir, report); err != nil {
			report.Error = err.Error()
			return report, err
		}
	}
	return report, nil
}

func saveReport(dir string, report *Report) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ReportPath(dir, report.Package), data, 0o644)
}

// ReportPath returns where the report for pkg would be stored.
func ReportPath(dir, pkg string) string {
	return filepath.Join(dir, pkg+".scan.json")
}
