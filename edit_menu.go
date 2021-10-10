// edit menu
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

// Editor returns the preferred system editor.
func editor() (editor string, args []string) {
	switch {
	case config.Editor != "":
		editor, err := exec.LookPath(config.Editor)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, strings.Fields(config.EditorFlags)
		}

		fallthrough
	case os.Getenv("EDITOR") != "":
		if editorArgs := strings.Fields(os.Getenv("EDITOR")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	case os.Getenv("VISUAL") != "":
		if editorArgs := strings.Fields(os.Getenv("VISUAL")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	default:
		fmt.Fprintln(os.Stderr)
		text.Errorln(gotext.Get("%s is not set", text.Bold(text.Cyan("$EDITOR"))))
		text.Warnln(gotext.Get("Add %s or %s to your environment variables", text.Bold(text.Cyan("$EDITOR")), text.Bold(text.Cyan("$VISUAL"))))

		for {
			text.Infoln(gotext.Get("Edit PKGBUILD with?"))

			editorInput, err := getInput("")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			editorArgs := strings.Fields(editorInput)
			if len(editorArgs) == 0 {
				continue
			}

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			return editor, editorArgs[1:]
		}
	}
}

func editNumberMenu(bases []dep.Base, installed stringset.StringSet) ([]dep.Base, error) {
	return editDiffNumberMenu(bases, installed, false)
}

func editDiffNumberMenu(bases []dep.Base, installed stringset.StringSet, diff bool) ([]dep.Base, error) {
	var (
		toEdit    = make([]dep.Base, 0)
		editInput string
		err       error
	)

	if diff {
		text.Infoln(gotext.Get("Diffs to show?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))

		editInput, err = getInput(config.AnswerDiff)
		if err != nil {
			return nil, err
		}
	} else {
		text.Infoln(gotext.Get("PKGBUILDs to edit?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))
		editInput, err = getInput(config.AnswerEdit)
		if err != nil {
			return nil, err
		}
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := intrange.ParseNumberMenu(editInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.Get("abort") || eOtherInclude.Get("ab") {
		return nil, &settings.ErrUserAbort{}
	}

	if !eOtherInclude.Get("n") && !eOtherInclude.Get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := base.AnyIsInSet(installed)

			if !eIsInclude && eExclude.Get(len(bases)-i) {
				continue
			}

			if anyInstalled && (eOtherInclude.Get("i") || eOtherInclude.Get("installed")) {
				toEdit = append(toEdit, base)
				continue
			}

			if !anyInstalled && (eOtherInclude.Get("no") || eOtherInclude.Get("notinstalled")) {
				toEdit = append(toEdit, base)
				continue
			}

			if eOtherInclude.Get("a") || eOtherInclude.Get("all") {
				toEdit = append(toEdit, base)
				continue
			}

			if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}

			if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}
		}
	}

	return toEdit, nil
}

func editPkgbuilds(bases []dep.Base, srcinfos map[string]*gosrc.Srcinfo) error {
	pkgbuilds := make([]string, 0, len(bases))

	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))

		for _, splitPkg := range srcinfos[pkg].SplitPackages() {
			if splitPkg.Install != "" {
				pkgbuilds = append(pkgbuilds, filepath.Join(dir, splitPkg.Install))
			}
		}
	}

	if len(pkgbuilds) > 0 {
		editor, editorArgs := editor()
		editorArgs = append(editorArgs, pkgbuilds...)
		editcmd := exec.Command(editor, editorArgs...)
		editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr

		if err := editcmd.Run(); err != nil {
			return errors.New(gotext.Get("editor did not exit successfully, aborting: %s", err))
		}
	}

	return nil
}

func editMenu(editMenuOption bool, bases []dep.Base, installed stringset.StringSet, srcinfos map[string]*gosrc.Srcinfo) error {
	if !editMenuOption {
		return nil
	}

	pkgbuildNumberMenu(bases, installed)

	toEdit, errMenu := editNumberMenu(bases, installed)
	if errMenu != nil || len(toEdit) == 0 {
		return errMenu
	}

	if errEdit := editPkgbuilds(toEdit, srcinfos); errEdit != nil {
		return errEdit
	}

	fmt.Println()

	if !text.ContinueTask(gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	return nil
}
