package query

import (
	"fmt"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
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
	if len(warnings.Missing) > 0 {
		text.Warn(gotext.Get("Missing AUR Packages:"))
		printRange(warnings.Missing)
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

func printRange(names []string) {
	for _, name := range names {
		fmt.Print("  " + text.Cyan(name))
	}
	fmt.Println()
}
