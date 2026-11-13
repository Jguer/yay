package menus

import (
	"fmt"
	"os"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/intrange"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"

	mapset "github.com/deckarep/golang-set/v2"
)

func pkgbuildNumberMenu(logger *text.Logger, pkgbuildDirs map[string]string,
	bases []string, installed mapset.Set[string],
) {
	var toPrint strings.Builder

	for n, pkgBase := range bases {
		dir := pkgbuildDirs[pkgBase]
		toPrint.WriteString(fmt.Sprintf(text.Magenta("%3d")+" %-40s", len(pkgbuildDirs)-n,
			text.Bold(pkgBase)))

		if installed.Contains(pkgBase) {
			toPrint.WriteString(text.Bold(text.Green(gotext.Get(" (Installed)"))))
		}

		// TODO: remove or refactor to check if git dir is unclean
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint.WriteString(text.Bold(text.Green(gotext.Get(" (Build Files Exist)"))))
		}

		toPrint.WriteString("\n")
	}

	logger.Print(toPrint.String())
}

func selectionMenu(logger *text.Logger, pkgbuildDirs map[string]string, bases []string, installed mapset.Set[string],
	message string, noConfirm bool, defaultAnswer string, skipFunc func(string) bool,
) ([]string, error) {
	selected := make([]string, 0)

	pkgbuildNumberMenu(logger, pkgbuildDirs, bases, installed)

	logger.Infoln(message)
	menuPrompt := gotext.Get("%s [A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)",
		text.Cyan(gotext.Get("[N]one")))
	logger.Infoln(menuPrompt)
	aliases := localizedMenuAliases(menuPrompt)

	selectInput, err := logger.GetInput(defaultAnswer, noConfirm)
	if err != nil {
		return nil, err
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := intrange.ParseNumberMenu(selectInput)
	eIsInclude := len(eExclude) == 0 && eOtherExclude.Cardinality() == 0

	if menuAliasSelected(eOtherInclude, aliases, "abort") ||
		menuAliasSelected(eOtherInclude, aliases, "ab") {
		return nil, settings.ErrUserAbort{}
	}

	if menuAliasSelected(eOtherInclude, aliases, "n") ||
		menuAliasSelected(eOtherInclude, aliases, "none") {
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

		if anyInstalled && (menuAliasSelected(eOtherInclude, aliases, "i") ||
			menuAliasSelected(eOtherInclude, aliases, "installed")) {
			selected = append(selected, pkgBase)
			continue
		}

		if !anyInstalled && (menuAliasSelected(eOtherInclude, aliases, "no") ||
			menuAliasSelected(eOtherInclude, aliases, "notinstalled")) {
			selected = append(selected, pkgBase)
			continue
		}

		if menuAliasSelected(eOtherInclude, aliases, "a") ||
			menuAliasSelected(eOtherInclude, aliases, "all") {
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

func localizedMenuAliases(prompt string) map[string][]string {
	aliases := map[string][]string{
		"all":          {"a"},
		"abort":        {"ab"},
		"installed":    {"i"},
		"notinstalled": {"no"},
		"none":         {"n"},
	}

	canonical := []string{"none", "all", "abort", "installed", "notinstalled"}
	for _, alias := range bracketedWords(prompt) {
		if len(canonical) == 0 {
			break
		}

		key := canonical[0]
		canonical = canonical[1:]
		aliases[key] = append(aliases[key], strings.ToLower(alias))
	}

	return aliases
}

func bracketedWords(s string) []string {
	var words []string
	for {
		start := strings.IndexByte(s, '[')
		if start < 0 {
			return words
		}

		s = s[start+1:]
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return words
		}

		if word := s[:end]; word != "" {
			words = append(words, word)
		}
		s = s[end+1:]
	}
}

func menuAliasSelected(input mapset.Set[string], aliases map[string][]string, canonical string) bool {
	if input.Contains(canonical) {
		return true
	}

	for _, alias := range aliases[canonical] {
		if input.Contains(alias) {
			return true
		}
	}

	return false
}
