// edit menu
package menus

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
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

// Editor returns the preferred system editor.
func editor(editorConfig, editorFlags string, noConfirm bool) (editor string, args []string) {
	switch {
	case editorConfig != "":
		editor, err := exec.LookPath(editorConfig)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, strings.Fields(editorFlags)
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

			editorInput, err := text.GetInput("", noConfirm)
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

func editPkgbuilds(buildDir string, bases []dep.Base, editorConfig,
	editorFlags string, srcinfos map[string]*gosrc.Srcinfo, noConfirm bool) error {
	pkgbuilds := make([]string, 0, len(bases))

	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(buildDir, pkg)
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))

		for _, splitPkg := range srcinfos[pkg].SplitPackages() {
			if splitPkg.Install != "" {
				pkgbuilds = append(pkgbuilds, filepath.Join(dir, splitPkg.Install))
			}
		}
	}

	if len(pkgbuilds) > 0 {
		editor, editorArgs := editor(editorConfig, editorFlags, noConfirm)
		editorArgs = append(editorArgs, pkgbuilds...)
		editcmd := exec.Command(editor, editorArgs...)
		editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr

		if err := editcmd.Run(); err != nil {
			return errors.New(gotext.Get("editor did not exit successfully, aborting: %s", err))
		}
	}

	return nil
}

func Edit(editMenuOption bool, buildDir string, bases []dep.Base, editorConfig,
	editorFlags string, installed stringset.StringSet, srcinfos map[string]*gosrc.Srcinfo,
	noConfirm bool, editDefaultAnswer string) error {
	if !editMenuOption {
		return nil
	}

	pkgbuildNumberMenu(buildDir, bases, installed)

	toEdit, errMenu := editDiffNumberMenu(bases, installed, gotext.Get("PKGBUILDs to edit?"), noConfirm, editDefaultAnswer)
	if errMenu != nil || len(toEdit) == 0 {
		return errMenu
	}

	if errEdit := editPkgbuilds(buildDir, toEdit, editorConfig, editorFlags, srcinfos, noConfirm); errEdit != nil {
		return errEdit
	}

	fmt.Println()

	if !text.ContinueTask(gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	return nil
}
