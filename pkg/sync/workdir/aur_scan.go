package workdir

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Jguer/yay/v13/pkg/scan"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"

	"github.com/leonelquinteros/gotext"
)

// ScanPackage audits one AUR pkgbase, reusing a cached report when the package
// contents and scanner version are unchanged. The report is always printed;
// callers decide what the verdict means for their operation.
func ScanPackage(ctx context.Context, cfg *settings.Configuration, logger *text.Logger,
	base, dir string,
) (*scan.Report, error) {
	reportDir := filepath.Join(cfg.BuildDir, "scan")
	if hash, err := scan.ContentHash(dir); err == nil {
		if cached := scan.LoadCached(reportDir, base, hash); cached != nil {
			cached.Cached = true
			logger.OperationInfoln(gotext.Get("Scanning %s for malware (cached result)", base))
			logger.Print(cached.Format())
			return cached, nil
		}
	}

	logger.OperationInfoln(gotext.Get("Scanning %s for malware", base))

	report, err := scan.New(scan.Options{
		Package:   base,
		Dir:       dir,
		ReportDir: reportDir,
	}).Run(ctx)

	logger.Print(report.Format())
	logger.Infoln(gotext.Get("scan report: %s", scan.ReportPath(reportDir, base)))
	return report, err
}

// scanAURPackages gates the install on the scan verdict: a malicious verdict
// always aborts, a suspicious verdict requires explicit confirmation.
func (preper *Preparer) scanAURPackages(ctx context.Context, pkgbuildDirsByBase map[string]string) error {
	if !preper.cfg.Scan || len(pkgbuildDirsByBase) == 0 {
		return nil
	}

	for _, base := range sortedAURBases(pkgbuildDirsByBase) {
		dir, _, _ := aurPreInstallPaths(pkgbuildDirsByBase[base])

		report, err := ScanPackage(ctx, preper.cfg, preper.log, base, dir)

		switch report.Verdict {
		case scan.VerdictMalicious:
			return fmt.Errorf("%s", gotext.Get("refusing to build %s: malware indicators found (inspect the report, or use --noscan to override)", base))
		case scan.VerdictSuspicious:
			question := gotext.Get("%s has suspicious findings. Continue building?", base)
			if !preper.log.ContinueTask(question, false, settings.NoConfirm) {
				return fmt.Errorf("%s", gotext.Get("aborted: suspicious scan verdict for %s", base))
			}
		default:
		}

		if err != nil {
			if preper.cfg.ScanStrict {
				return fmt.Errorf("%s: %w", gotext.Get("scan failed for %s and --scanstrict is set", base), err)
			}
			preper.log.Warnln(gotext.Get("scan incomplete for %s: %s", base, err))
		}
	}
	return nil
}
