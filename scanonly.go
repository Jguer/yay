package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/scan"
	"github.com/Jguer/yay/v13/pkg/sync/workdir"
)

// yay -Y --scanonly <pkgbase...>: download and audit AUR packages without
// building or installing anything. Exits non-zero when any package is
// malicious; suspicious and incomplete scans are reported but do not prompt.
func scanOnly(ctx context.Context, run *runtime.Runtime, targets []string) error {
	if len(targets) == 0 {
		return errors.New(gotext.Get("no targets specified for --scanonly"))
	}

	logger := run.Logger.Child("scan")
	if err := os.MkdirAll(run.Cfg.BuildDir, 0o755); err != nil {
		return err
	}
	if _, err := download.AURPKGBUILDRepos(ctx, run.CmdBuilder, logger,
		targets, run.Cfg.AURURL, run.Cfg.BuildDir, false); err != nil {
		return err
	}

	dirs := make(map[string]string, len(targets))
	for _, base := range targets {
		dirs[base] = filepath.Join(run.Cfg.BuildDir, base)
	}
	if err := workdir.DownloadPKGBUILDSourceFanout(ctx, run.CmdBuilder, dirs, false,
		run.Cfg.MaxConcurrentDownloads); err != nil {
		return err
	}

	malicious := []string{}
	for _, base := range targets {
		report, err := workdir.ScanPackage(ctx, run.Cfg, logger, base, dirs[base])
		if report.Verdict == scan.VerdictMalicious {
			malicious = append(malicious, base)
		}
		if err != nil {
			logger.Warnln(gotext.Get("scan incomplete for %s: %s", base, err))
		}
	}

	if len(malicious) > 0 {
		return fmt.Errorf("%s", gotext.Get("malware indicators found in: %v", malicious))
	}
	return nil
}
