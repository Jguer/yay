// edit menu
package menus

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
)

// Editor returns the preferred system editor.
func editor(log *text.Logger, editorConfig, editorFlags string, noConfirm bool) (editor string, args []string) {
	switch {
	case editorConfig != "":
		editor, err := exec.LookPath(editorConfig)
		if err != nil {
			log.Errorln(err)
		} else {
			return editor, strings.Fields(editorFlags)
		}

		fallthrough
	case os.Getenv("VISUAL") != "":
		if editorArgs := strings.Fields(os.Getenv("VISUAL")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				log.Errorln(err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	case os.Getenv("EDITOR") != "":
		if editorArgs := strings.Fields(os.Getenv("EDITOR")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				log.Errorln(err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	default:
		log.Errorln("\n", gotext.Get("%s is not set", text.Bold(text.Cyan("$EDITOR"))))
		log.Warnln(gotext.Get("Add %s or %s to your environment variables", text.Bold(text.Cyan("$EDITOR")), text.Bold(text.Cyan("$VISUAL"))))

		for {
			log.Infoln(gotext.Get("Edit PKGBUILD with?"))

			editorInput, err := text.GetInput(os.Stdin, "", noConfirm)
			if err != nil {
				log.Errorln(err)
				continue
			}

			editorArgs := strings.Fields(editorInput)
			if len(editorArgs) == 0 {
				continue
			}

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				log.Errorln(err)
				continue
			}

			return editor, editorArgs[1:]
		}
	}
}

func editPkgbuilds(log *text.Logger, pkgbuildDirs map[string]string, bases []string, editorConfig,
	editorFlags string, srcinfos map[string]*gosrc.Srcinfo, noConfirm bool,
) error {
	pkgbuilds := make([]string, 0, len(bases))

	for _, pkg := range bases {
		dir := pkgbuildDirs[pkg]
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))

		if srcinfos != nil {
			for _, splitPkg := range srcinfos[pkg].SplitPackages() {
				if splitPkg.Install != "" {
					pkgbuilds = append(pkgbuilds, filepath.Join(dir, splitPkg.Install))
				}
			}
		}
	}

	if len(pkgbuilds) > 0 {
		editor, editorArgs := editor(log, editorConfig, editorFlags, noConfirm)
		editorArgs = append(editorArgs, pkgbuilds...)
		editcmd := exec.Command(editor, editorArgs...)
		editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr

		if err := editcmd.Run(); err != nil {
			return errors.New(gotext.Get("editor did not exit successfully, aborting: %s", err))
		}
	}

	return nil
}

func EditFn(ctx context.Context, cfg *settings.Configuration, w io.Writer,
	pkgbuildDirsByBase map[string]string,
) error {
	if len(pkgbuildDirsByBase) == 0 {
		return nil // no work to do
	}

	bases := make([]string, 0, len(pkgbuildDirsByBase))
	for pkg := range pkgbuildDirsByBase {
		bases = append(bases, pkg)
	}

	toEdit, errMenu := selectionMenu(w, pkgbuildDirsByBase, bases,
		mapset.NewThreadUnsafeSet[string](),
		gotext.Get("PKGBUILDs to edit?"), settings.NoConfirm, cfg.AnswerEdit, nil)
	if errMenu != nil || len(toEdit) == 0 {
		return errMenu
	}

	// TOFIX: remove or use srcinfo data
	if errEdit := editPkgbuilds(cfg.Runtime.Logger, pkgbuildDirsByBase,
		toEdit, cfg.Editor, cfg.EditorFlags, nil, settings.NoConfirm); errEdit != nil {
		return errEdit
	}

	cfg.Runtime.Logger.Println()

	if !text.ContinueTask(os.Stdin, gotext.Get("Proceed with install?"), true, false) {
		return settings.ErrUserAbort{}
	}

	return nil
}
