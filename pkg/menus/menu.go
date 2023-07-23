package menus

import (
	"fmt"
	"io"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"

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

		// TODO: remove or refactor to check if git dir is unclean
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

	selectInput, err := text.GetInput(os.Stdin, defaultAnswer, noConfirm)
	if err != nil {
		return nil, err
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := intrange.ParseNumberMenu(selectInput)
	eIsInclude := len(eExclude) == 0 && eOtherExclude.Cardinality() == 0

	if eOtherInclude.Contains("abort") || eOtherInclude.Contains("ab") {
		return nil, settings.ErrUserAbort{}
	}

	if eOtherInclude.Contains("n") || eOtherInclude.Contains("none") {
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

		if anyInstalled && (eOtherInclude.Contains("i") || eOtherInclude.Contains("installed")) {
			selected = append(selected, pkgBase)
			continue
		}

		if !anyInstalled && (eOtherInclude.Contains("no") || eOtherInclude.Contains("notinstalled")) {
			selected = append(selected, pkgBase)
			continue
		}

		if eOtherInclude.Contains("a") || eOtherInclude.Contains("all") {
			selected = append(selected, pkgBase)
			continue
		}

		if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Contains(pkgBase)) {
			selected = append(selected, pkgBase)
		}

		if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Contains(pkgBase)) {
			selected = append(selected, pkgBase)
		}
	}

	return selected, nil
}
