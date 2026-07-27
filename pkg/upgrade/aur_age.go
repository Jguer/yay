package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gosrc "github.com/Morganamilo/go-srcinfo"

	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

func findAURVersionBefore(ctx context.Context, cmdBuilder exe.ICmdBuilder, aurURL, buildDir, base string,
	minAgeDays int,
) (gitRef, version string, lastModified int64, ok bool) {
	if minAgeDays <= 0 || base == "" {
		return "", "", 0, false
	}

	pkgDir := filepath.Join(buildDir, base)
	if _, err := download.AURPKGBUILDRepo(ctx, cmdBuilder, aurURL, base, buildDir, false); err != nil {
		return "", "", 0, false
	}

	cutoff := text.NowFunc().Add(-time.Duration(minAgeDays) * 24 * time.Hour)
	before := cutoff.UTC().Format(time.RFC3339)

	logCmd := cmdBuilder.BuildGitCmd(ctx, pkgDir, "log", "--before="+before, "-1", "--format=%at %H")
	stdout, _, err := cmdBuilder.Capture(logCmd)
	if err != nil {
		return "", "", 0, false
	}

	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) != 2 {
		return "", "", 0, false
	}

	ts, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return "", "", 0, false
	}

	commit := fields[1]
	showCmd := cmdBuilder.BuildGitCmd(ctx, pkgDir, "show", commit+":.SRCINFO")
	srcinfoOut, _, err := cmdBuilder.Capture(showCmd)
	if err != nil {
		return "", "", 0, false
	}

	srcinfo, err := gosrc.Parse(srcinfoOut)
	if err != nil {
		return "", "", 0, false
	}

	return commit, srcinfo.Version(), ts, true
}

// CheckoutAURGitRef checks out a pinned AUR commit in an existing clone.
func CheckoutAURGitRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgDir, gitRef string) error {
	if gitRef == "" {
		return nil
	}

	if _, err := os.Stat(filepath.Join(pkgDir, ".git")); err != nil {
		return fmt.Errorf("aur git ref %s: %w", gitRef, err)
	}

	cmd := cmdBuilder.BuildGitCmd(ctx, pkgDir, "checkout", gitRef)
	_, stderr, err := cmdBuilder.Capture(cmd)
	if err != nil {
		return fmt.Errorf("%w: %s", err, stderr)
	}

	return nil
}
