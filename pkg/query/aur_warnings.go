package query

import (
	"fmt"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

type AURWarnings struct {
	Orphans   []string
	OutOfDate []string
	Missing   []string
	Ignore    stringset.StringSet
}

func NewWarnings() *AURWarnings {
	return &AURWarnings{Ignore: make(stringset.StringSet)}
}

func (warnings *AURWarnings) Print() {
	normalMissing, debugMissing := filterDebugPkgs(warnings.Missing)

	if len(normalMissing) > 0 {
		text.Warn(gotext.Get("Missing AUR Packages:"))
		printRange(normalMissing)
	}

	if len(debugMissing) > 0 {
		text.Warn(gotext.Get("Missing AUR Debug Packages:"))
		printRange(debugMissing)
	}

	if len(warnings.Orphans) > 0 {
		text.Warn(gotext.Get("Orphaned AUR Packages:"))
		printRange(warnings.Orphans)
	}

	if len(warnings.OutOfDate) > 0 {
		text.Warn(gotext.Get("Flagged Out Of Date AUR Packages:"))
		printRange(warnings.OutOfDate)
	}
}

func filterDebugPkgs(names []string) (normal, debug []string) {
	normal = make([]string, 0, len(names))
	debug = make([]string, 0, len(names))

	for _, name := range names {
		if strings.HasSuffix(name, "-debug") {
			debug = append(debug, name)
		} else {
			normal = append(normal, name)
		}
	}

	return
}

func printRange(names []string) {
	for _, name := range names {
		fmt.Print("  " + text.Cyan(name))
	}

	fmt.Println()
}
