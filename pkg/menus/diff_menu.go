// file dedicated to diff menu
package menus

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/text"
)

const (
	gitEmptyTree   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	gitDiffRefName = "AUR_SEEN"
)

func showPkgbuildDiffs(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	pkgbuildDirs map[string]string, bases []string,
) error {
	var errMulti multierror.MultiError

	for _, pkg := range bases {
		dir := pkgbuildDirs[pkg]

		start, err := getLastSeenHash(ctx, cmdBuilder, dir)
		if err != nil {
			errMulti.Add(err)

			continue
		}

		if start != gitEmptyTree {
			hasDiff, err := gitHasDiff(ctx, cmdBuilder, dir)
			if err != nil {
				errMulti.Add(err)

				continue
			}

			if !hasDiff {
				text.Warnln(gotext.Get("%s: No changes -- skipping", text.Cyan(pkg)))

				continue
			}
		}

		args := []string{
			"diff",
			start + "..HEAD@{upstream}", "--src-prefix",
			dir + "/", "--dst-prefix", dir + "/", "--", ".", ":(exclude).SRCINFO",
		}
		if text.UseColor {
			args = append(args, "--color=always")
		} else {
			args = append(args, "--color=never")
		}

		_ = cmdBuilder.Show(cmdBuilder.BuildGitCmd(ctx, dir, args...))
	}

	return errMulti.Return()
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
	// If YAY_DIFF_REVIEW does not exists, we have never reviewed a diff for this package
	// and should display it.
	return true, nil
}

// Return whether or not we have reviewed a diff yet. It checks for the existence of
// YAY_DIFF_REVIEW in the git ref-list.
func gitHasLastSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) bool {
	_, _, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "rev-parse", "--quiet", "--verify", gitDiffRefName))

	return err == nil
}

// Returns the last reviewed hash. If YAY_DIFF_REVIEW exists it will return this hash.
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

// Update the YAY_DIFF_REVIEW ref to HEAD. We use this ref to determine which diff were
// reviewed by the user.
func gitUpdateSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, dir string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			dir, "update-ref", gitDiffRefName, "HEAD"))
	if err != nil {
		return fmt.Errorf("%s %w", stderr, err)
	}

	return nil
}

func updatePkgbuildSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, pkgbuildDirs map[string]string, bases []string) error {
	var errMulti multierror.MultiError

	for _, pkg := range bases {
		dir := pkgbuildDirs[pkg]
		if err := gitUpdateSeenRef(ctx, cmdBuilder, dir); err != nil {
			errMulti.Add(err)
		}
	}

	return errMulti.Return()
}

func DiffFn(ctx context.Context, run *settings.Runtime, w io.Writer,
	pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
) error {
	if len(pkgbuildDirsByBase) == 0 {
		return nil // no work to do
	}

	bases := make([]string, 0, len(pkgbuildDirsByBase))
	for base := range pkgbuildDirsByBase {
		bases = append(bases, base)
	}

	toDiff, errMenu := selectionMenu(w, pkgbuildDirsByBase, bases, installed, gotext.Get("Diffs to show?"),
		settings.NoConfirm, run.Cfg.AnswerDiff, nil)
	if errMenu != nil || len(toDiff) == 0 {
		return errMenu
	}

	if errD := showPkgbuildDiffs(ctx, run.CmdBuilder, pkgbuildDirsByBase, toDiff); errD != nil {
		return errD
	}

	fmt.Println()

	if !text.ContinueTask(os.Stdin, gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	if errUpd := updatePkgbuildSeenRef(ctx, run.CmdBuilder, pkgbuildDirsByBase, toDiff); errUpd != nil {
		return errUpd
	}

	return nil
}
