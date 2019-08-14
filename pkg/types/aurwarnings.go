package types

import (
	"fmt"

	"github.com/Jguer/yay/v10/pkg/text"
)

// AURWarnings holds package issuees found during AUR search
type AURWarnings struct {
	Orphans   []string
	OutOfDate []string
	Missing   []string
}

const smallArrow = " ->"
const arrow = "==>"

// Print prints AURWarnings returned from AUR operations
func (warnings *AURWarnings) Print() {
	if len(warnings.Missing) > 0 {
		fmt.Print(text.Bold(text.Yellow(smallArrow)) + " Missing AUR Packages:")
		for _, name := range warnings.Missing {
			fmt.Print("  " + text.Cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.Orphans) > 0 {
		fmt.Print(text.Bold(text.Yellow(smallArrow)) + " Orphaned AUR Packages:")
		for _, name := range warnings.Orphans {
			fmt.Print("  " + text.Cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.OutOfDate) > 0 {
		fmt.Print(text.Bold(text.Yellow(smallArrow)) + " Out Of Date AUR Packages:")
		for _, name := range warnings.OutOfDate {
			fmt.Print("  " + text.Cyan(name))
		}
		fmt.Println()
	}

}
