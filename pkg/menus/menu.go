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

func selectionMenu(buildDir string, bases []dep.Base, installed stringset.StringSet,
	message string, noConfirm bool, defaultAnswer string, skipFunc func(string) bool) ([]dep.Base, error) {
	selected := make([]dep.Base, 0)

	pkgbuildNumberMenu(buildDir, bases, installed)

	text.Infoln(message)
	text.Infoln(gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)", text.Cyan(gotext.Get("[N]one"))))

	selectInput, err := text.GetInput(defaultAnswer, noConfirm)
	if err != nil {
		return nil, err
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := intrange.ParseNumberMenu(selectInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.Get("abort") || eOtherInclude.Get("ab") {
		return nil, settings.ErrUserAbort{}
	}

	if eOtherInclude.Get("n") || eOtherInclude.Get("none") {
		return selected, nil
	}

	for i, base := range bases {
		pkg := base.Pkgbase()

		if skipFunc != nil && skipFunc(pkg) {
			continue
		}

		anyInstalled := base.AnyIsInSet(installed)

		if !eIsInclude && eExclude.Get(len(bases)-i) {
			continue
		}

		if anyInstalled && (eOtherInclude.Get("i") || eOtherInclude.Get("installed")) {
			selected = append(selected, base)
			continue
		}

		if !anyInstalled && (eOtherInclude.Get("no") || eOtherInclude.Get("notinstalled")) {
			selected = append(selected, base)
			continue
		}

		if eOtherInclude.Get("a") || eOtherInclude.Get("all") {
			selected = append(selected, base)
			continue
		}

		if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Get(pkg)) {
			selected = append(selected, base)
		}

		if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Get(pkg)) {
			selected = append(selected, base)
		}
	}

	return selected, nil
}
