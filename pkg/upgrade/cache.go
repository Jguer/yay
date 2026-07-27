package upgrade

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

func pacmanQueryFile(ctx context.Context, cmdBuilder exe.ICmdBuilder, pacmanBin, pkgPath string) (
	name, version string, buildTime int64, err error,
) {
	cmd := exec.CommandContext(ctx, pacmanBin, "-Qp", "--print-format", "%n|%v|%t", pkgPath)
	stdout, _, err := cmdBuilder.Capture(cmd)
	if err != nil {
		return "", "", 0, err
	}

	parts := strings.Split(strings.TrimSpace(stdout), "|")
	if len(parts) != 3 {
		return "", "", 0, fmt.Errorf("unexpected pacman -Qp output: %q", stdout)
	}

	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", "", 0, err
	}

	return parts[0], parts[1], ts, nil
}

// FindEligibleCachePackage returns the newest pacman-cached package at least minAgeDays old.
func FindEligibleCachePackage(ctx context.Context, cmdBuilder exe.ICmdBuilder, pacmanBin string, cacheDirs []string,
	pkgName string, minAgeDays int,
) (path, version string, buildTime int64, ok bool) {
	if minAgeDays <= 0 || len(cacheDirs) == 0 || pacmanBin == "" {
		return "", "", 0, false
	}

	cutoff := text.NowFunc().Add(-time.Duration(minAgeDays) * 24 * time.Hour)
	var bestPath, bestVersion string
	var bestTime int64

	for _, dir := range cacheDirs {
		matches, err := filepath.Glob(filepath.Join(dir, pkgName+"-*.pkg.tar*"))
		if err != nil {
			continue
		}

		for _, candidate := range matches {
			name, ver, built, errQ := pacmanQueryFile(ctx, cmdBuilder, pacmanBin, candidate)
			if errQ != nil || name != pkgName || built == 0 || time.Unix(built, 0).After(cutoff) {
				continue
			}

			if bestPath == "" || db.VerCmp(ver, bestVersion) > 0 {
				bestPath, bestVersion, bestTime = candidate, ver, built
			}
		}
	}

	if bestPath == "" {
		return "", "", 0, false
	}

	return bestPath, bestVersion, bestTime, true
}
