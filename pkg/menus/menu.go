package menus

import (
	"fmt"
	"io"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/text"

	mapset "github.com/deckarep/golang-set/v2"
)

func pkgbuildNumberMenu(w io.Writer, pkgbuildDirs map[string]string, bases []string, installed mapset.Set[string]) {
	toPrint := ""

	for n, pkgBase := range bases {
		dir := pkgbuildDirs[pkgBase]
		toPrint += fmt.Sprintf(text.Magenta("%3d")+" %-40s", len(pkgbuildDirs)-n,
			text.Bold(pkgBase))

		if installed.Contains(pkgBase) {
			toPrint += text.Bold(text.Green(gotext.Get(" (Installed)")))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += text.Bold(text.Green(gotext.Get(" (Build Files Exist)")))
		}

		toPrint += "\n"
	}

	fmt.Fprint(w, toPrint)
}

func selectionMenu(w io.Writer, pkgbuildDirs map[string]string, bases []string, installed mapset.Set[string],
	message string, noConfirm bool, defaultAnswer string, skipFunc func(string) bool,
) ([]string, error) {
	selected := make([]string, 0)

	pkgbuildNumberMenu(w, pkgbuildDirs, bases, installed)

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

	for i, pkgBase := range bases {
		if skipFunc != nil && skipFunc(pkgBase) {
			continue
		}

		anyInstalled := installed.Contains(pkgBase)

		if !eIsInclude && eExclude.Get(len(bases)-i) {
			continue
		}

		if anyInstalled && (eOtherInclude.Get("i") || eOtherInclude.Get("installed")) {
			selected = append(selected, pkgBase)
			continue
		}

		if !anyInstalled && (eOtherInclude.Get("no") || eOtherInclude.Get("notinstalled")) {
			selected = append(selected, pkgBase)
			continue
		}

		if eOtherInclude.Get("a") || eOtherInclude.Get("all") {
			selected = append(selected, pkgBase)
			continue
		}

		if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Get(pkgBase)) {
			selected = append(selected, pkgBase)
		}

		if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Get(pkgBase)) {
			selected = append(selected, pkgBase)
		}
	}

	return selected, nil
}
