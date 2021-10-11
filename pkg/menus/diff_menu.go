// file dedicated to diff menu
package menus

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

const (
	gitEmptyTree   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	gitDiffRefName = "AUR_SEEN"
)

func showPkgbuildDiffs(ctx context.Context, cmdBuilder exe.ICmdBuilder, buildDir string, bases []dep.Base, cloned map[string]bool) error {
	var errMulti multierror.MultiError

	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(buildDir, pkg)

		start, err := getLastSeenHash(ctx, cmdBuilder, buildDir, pkg)
		if err != nil {
			errMulti.Add(err)

			continue
		}

		if cloned[pkg] {
			start = gitEmptyTree
		} else {
			hasDiff, err := gitHasDiff(ctx, cmdBuilder, buildDir, pkg)
			if err != nil {
				errMulti.Add(err)

				continue
			}

			if !hasDiff {
				text.Warnln(gotext.Get("%s: No changes -- skipping", text.Cyan(base.String())))

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
func gitHasDiff(ctx context.Context, cmdBuilder exe.ICmdBuilder, path, name string) (bool, error) {
	if gitHasLastSeenRef(ctx, cmdBuilder, path, name) {
		stdout, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(ctx, filepath.Join(path, name), "rev-parse", gitDiffRefName, "HEAD@{upstream}"))
		if err != nil {
			return false, fmt.Errorf("%s%s", stderr, err)
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

// Return wether or not we have reviewed a diff yet. It checks for the existence of
// YAY_DIFF_REVIEW in the git ref-list.
func gitHasLastSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, path, name string) bool {
	_, _, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "rev-parse", "--quiet", "--verify", gitDiffRefName))

	return err == nil
}

// Returns the last reviewed hash. If YAY_DIFF_REVIEW exists it will return this hash.
// If it does not it will return empty tree as no diff have been reviewed yet.
func getLastSeenHash(ctx context.Context, cmdBuilder exe.ICmdBuilder, path, name string) (string, error) {
	if gitHasLastSeenRef(ctx, cmdBuilder, path, name) {
		stdout, stderr, err := cmdBuilder.Capture(
			cmdBuilder.BuildGitCmd(ctx,
				filepath.Join(path, name), "rev-parse", gitDiffRefName))
		if err != nil {
			return "", fmt.Errorf("%s %s", stderr, err)
		}

		lines := strings.Split(stdout, "\n")

		return lines[0], nil
	}

	return gitEmptyTree, nil
}

// Update the YAY_DIFF_REVIEW ref to HEAD. We use this ref to determine which diff were
// reviewed by the user.
func gitUpdateSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, path, name string) error {
	_, stderr, err := cmdBuilder.Capture(
		cmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "update-ref", gitDiffRefName, "HEAD"))
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}

	return nil
}

func updatePkgbuildSeenRef(ctx context.Context, cmdBuilder exe.ICmdBuilder, buildDir string, bases []dep.Base) error {
	var errMulti multierror.MultiError

	for _, base := range bases {
		pkg := base.Pkgbase()

		if err := gitUpdateSeenRef(ctx, cmdBuilder, buildDir, pkg); err != nil {
			errMulti.Add(err)
		}
	}

	return errMulti.Return()
}

func Diff(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	buildDir string, diffMenuOption bool, bases []dep.Base,
	installed stringset.StringSet, cloned map[string]bool, noConfirm bool, diffDefaultAnswer string) error {
	if !diffMenuOption {
		return nil
	}

	toDiff, errMenu := selectionMenu(buildDir, bases, installed, gotext.Get("Diffs to show?"),
		noConfirm, diffDefaultAnswer, nil)
	if errMenu != nil || len(toDiff) == 0 {
		return errMenu
	}

	if errD := showPkgbuildDiffs(ctx, cmdBuilder, buildDir, toDiff, cloned); errD != nil {
		return errD
	}

	fmt.Println()

	if !text.ContinueTask(gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	if errUpd := updatePkgbuildSeenRef(ctx, cmdBuilder, buildDir, toDiff); errUpd != nil {
		return errUpd
	}

	return nil
}
