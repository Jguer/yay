package query

import (
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func RemoveInvalidTargets(targets []string, mode parser.TargetMode) []string {
	filteredTargets := make([]string, 0)

	for _, target := range targets {
		dbName, _ := text.SplitDBFromName(target)

		if dbName == "aur" && !mode.AtLeastAUR() {
			text.Warnln(gotext.Get("%s: can't use target with option --repo -- skipping", text.Cyan(target)))
			continue
		}

		if dbName != "aur" && dbName != "" && !mode.AtLeastRepo() {
			text.Warnln(gotext.Get("%s: can't use target with option --aur -- skipping", text.Cyan(target)))
			continue
		}

		filteredTargets = append(filteredTargets, target)
	}

	return filteredTargets
}
