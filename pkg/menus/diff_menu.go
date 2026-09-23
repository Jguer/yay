// file dedicated to diff menu
package menus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

const (
	gitEmptyTree   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	gitDiffRefName = "AUR_SEEN"
)

func showPkgbuildDiffs(ctx context.Context, cmdBuilder exe.ICmdBuilder, logger *text.Logger,
	pkgbuildDirs map[string]string, bases []string, pagerConfig string,
) error {
	combined, errs := collectPkgbuildDiffs(ctx, cmdBuilder, logger, pkgbuildDirs, bases)
	if combined == "" {
		return errors.Join(errs...)
	}

	if !isStdoutTerminal() {
		logger.Print(combined)

		return errors.Join(errs...)
	}

	if err := runPager(ctx, combined, pagerConfig); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// collectPkgbuildDiffs captures each selected package's git diff with --no-pager
// and joins them with package headers into one buffer for a single pager session.
func collectPkgbuildDiffs(ctx context.Context, cmdBuilder exe.ICmdBuilder, logger *text.Logger,
	pkgbuildDirs map[string]string, bases []string,
) (string, []error) {
	var (
		errs []error
		buf  strings.Builder
	)

	for _, pkg := range bases {
		dir := pkgbuildDirs[pkg]

		start, err := getLastSeenHash(ctx, cmdBuilder, dir)
		if err != nil {
			errs = append(errs, err)

			continue
		}

		if start != gitEmptyTree {
			hasDiff, err := gitHasDiff(ctx, cmdBuilder, dir)
			if err != nil {
				errs = append(errs, err)

				continue
			}

			if !hasDiff {
				logger.Warnln(gotext.Get("%s: No changes -- skipping", text.Cyan(pkg)))

				continue
			}
		}

		args := []string{"--no-pager", "diff"}
		if text.UseColor {
			args = append(args, "--color=always")
		} else {
			args = append(args, "--color=never")
		}

		args = append(args,
			start+"..HEAD@{upstream}", "--src-prefix",
			dir+"/", "--dst-prefix", dir+"/", "--", ".", ":(exclude).SRCINFO",
		)

		stdout, stderr, err := cmdBuilder.Capture(cmdBuilder.BuildGitCmd(ctx, dir, args...))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s%w", stderr, err))

			continue
		}

		if stdout == "" {
			continue
		}

		buf.WriteString(logger.SprintOperationInfo(gotext.Get("Showing diff for %s", text.Bold(pkg))))
		buf.WriteByte('\n')
		buf.WriteString(stdout)
		buf.WriteString("\n\n")
	}

	return buf.String(), errs
}

// Check whether or not a diff exists between the last reviewed diff and
// HEAD@{upstream}.
func gitHasDiff(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) (bool, error) {
	if gitHasLastSeenRef(ctx, cmdBuilder, dir) {
		stdout, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(ctx, dir, "rev-parse", gitDiffRefName, "HEAD@{upstream}"))
		if err != nil {
			return false, fmt.Errorf("%s%w", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		lastseen := lines[0]
		upstream := lines[1]

		return lastseen != upstream, nil
	}
	// If AUR_SEEN does not exists, we have never reviewed a diff for this package
	// and should display it.
	return true, nil
}

// Return whether or not we have reviewed a diff yet. It checks for the existence of
// AUR_SEEN in the git ref-list.
func gitHasLastSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) bool {
	_, _, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "rev-parse", "--quiet", "--verify", gitDiffRefName))

	return err == nil
}

// Returns the last reviewed hash. If AUR_SEEN exists it will return this hash.
// If it does not it will return empty tree as no diff have been reviewed yet.
func getLastSeenHash(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) (string, error) {
	if gitHasLastSeenRef(ctx, cmdBuilder, dir) {
		stdout, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(ctx,
				dir, "rev-parse", gitDiffRefName))
		if err != nil {
			return "", fmt.Errorf("%s %w", stderr, err)
		}

		lines := strings.Split(stdout, "\n")

		return lines[0], nil
	}

	return gitEmptyTree, nil
}

// Update the AUR_SEEN ref to the upstream HEAD. We use this ref to determine
// which upstream diff was reviewed by the user, excluding local commits.
func gitUpdateSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "update-ref", gitDiffRefName, "HEAD@{upstream}"))
	if err != nil {
		return fmt.Errorf("%s %w", stderr, err)
	}

	return nil
}

func updatePkgbuildSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string, bases []string) error {
	var errs []error

	for _, pkg := range bases {
		dir := pkgbuildDirs[pkg]
		if err := gitUpdateSeenRef(ctx, cmdBuilder, dir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func DiffFn(ctx context.Context, run *runtime.Runtime, w io.Writer,
	pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
) error {
	if len(pkgbuildDirsByBase) == 0 {
		return nil // no work to do
	}

	bases := make([]string, 0, len(pkgbuildDirsByBase))
	for base := range pkgbuildDirsByBase {
		bases = append(bases, base)
	}

	toDiff, errMenu := selectionMenu(run.Logger, pkgbuildDirsByBase, bases, installed, gotext.Get("Diffs to show?"),
		settings.NoConfirm, run.Cfg.AnswerDiff, nil)
	if errMenu != nil || len(toDiff) == 0 {
		return errMenu
	}

	if errD := showPkgbuildDiffs(ctx, run.CmdBuilder, run.Logger, pkgbuildDirsByBase, toDiff, run.Cfg.Pager); errD != nil {
		return errD
	}

	run.Logger.Println()

	if !run.Logger.ContinueTask(gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	if errUpd := updatePkgbuildSeenRef(ctx, run.CmdBuilder, pkgbuildDirsByBase, toDiff); errUpd != nil {
		return errUpd
	}

	return nil
}
