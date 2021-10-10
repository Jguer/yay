// file dedicated to diff menu
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

const gitDiffRefName = "AUR_SEEN"

func showPkgbuildDiffs(ctx context.Context, bases []dep.Base, cloned map[string]bool) error {
	var errMulti multierror.MultiError

	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)

		start, err := getLastSeenHash(ctx, config.BuildDir, pkg)
		if err != nil {
			errMulti.Add(err)

			continue
		}

		if cloned[pkg] {
			start = gitEmptyTree
		} else {
			hasDiff, err := gitHasDiff(ctx, config.BuildDir, pkg)
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

		_ = config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildGitCmd(ctx, dir, args...))
	}

	return errMulti.Return()
}

// Check whether or not a diff exists between the last reviewed diff and
// HEAD@{upstream}.
func gitHasDiff(ctx context.Context, path, name string) (bool, error) {
	if gitHasLastSeenRef(ctx, path, name) {
		stdout, stderr, err := config.Runtime.CmdBuilder.Capture(
			config.Runtime.CmdBuilder.BuildGitCmd(ctx, filepath.Join(path, name), "rev-parse", gitDiffRefName, "HEAD@{upstream}"))
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
func gitHasLastSeenRef(ctx context.Context, path, name string) bool {
	_, _, err := config.Runtime.CmdBuilder.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "rev-parse", "--quiet", "--verify", gitDiffRefName))

	return err == nil
}

// Returns the last reviewed hash. If YAY_DIFF_REVIEW exists it will return this hash.
// If it does not it will return empty tree as no diff have been reviewed yet.
func getLastSeenHash(ctx context.Context, path, name string) (string, error) {
	if gitHasLastSeenRef(ctx, path, name) {
		stdout, stderr, err := config.Runtime.CmdBuilder.Capture(
			config.Runtime.CmdBuilder.BuildGitCmd(ctx,
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
func gitUpdateSeenRef(ctx context.Context, path, name string) error {
	_, stderr, err := config.Runtime.CmdBuilder.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(ctx,
			filepath.Join(path, name), "update-ref", gitDiffRefName, "HEAD"))
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}

	return nil
}

func diffNumberMenu(bases []dep.Base, installed stringset.StringSet) ([]dep.Base, error) {
	return editDiffNumberMenu(bases, installed, true)
}

func updatePkgbuildSeenRef(ctx context.Context, bases []dep.Base) error {
	var errMulti multierror.MultiError

	for _, base := range bases {
		pkg := base.Pkgbase()

		if err := gitUpdateSeenRef(ctx, config.BuildDir, pkg); err != nil {
			errMulti.Add(err)
		}
	}

	return errMulti.Return()
}

func diffMenu(ctx context.Context, diffMenuOption bool, bases []dep.Base, installed stringset.StringSet, cloned map[string]bool) error {
	if !diffMenuOption {
		return nil
	}

	pkgbuildNumberMenu(bases, installed)

	toDiff, errMenu := diffNumberMenu(bases, installed)
	if errMenu != nil || len(toDiff) == 0 {
		return errMenu
	}

	if errD := showPkgbuildDiffs(ctx, toDiff, cloned); errD != nil {
		return errD
	}

	fmt.Println()

	if !text.ContinueTask(gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	if errUpd := updatePkgbuildSeenRef(ctx, toDiff); errUpd != nil {
		return errUpd
	}

	return nil
}
