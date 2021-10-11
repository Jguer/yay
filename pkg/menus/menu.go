package menus

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

func pkgbuildNumberMenu(buildDir string, bases []dep.Base, installed stringset.StringSet) {
	toPrint := ""

	for n, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(buildDir, pkg)

		toPrint += fmt.Sprintf(text.Magenta("%3d")+" %-40s", len(bases)-n,
			text.Bold(base.String()))

		if base.AnyIsInSet(installed) {
			toPrint += text.Bold(text.Green(gotext.Get(" (Installed)")))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += text.Bold(text.Green(gotext.Get(" (Build Files Exist)")))
		}

		toPrint += "\n"
	}

	fmt.Print(toPrint)
}

func editDiffNumberMenu(bases []dep.Base, installed stringset.StringSet, diff, noConfirm bool, defaultAnswer string) ([]dep.Base, error) {
	var (
		toEdit    = make([]dep.Base, 0)
		editInput string
		err       error
	)

	if diff {
		text.Infoln(gotext.Get("Diffs to show?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))

		editInput, err = text.GetInput(defaultAnswer, noConfirm)
		if err != nil {
			return nil, err
		}
	} else {
		text.Infoln(gotext.Get("PKGBUILDs to edit?"))
		text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))
		editInput, err = text.GetInput(defaultAnswer, noConfirm)
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
