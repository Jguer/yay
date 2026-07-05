package query

import (
	"fmt"
	"strconv"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/text"
)

type Pkg = aur.Pkg

func getSearchBy(value string) aur.By {
	switch value {
	case "name":
		return aur.Name
	case "maintainer":
		return aur.Maintainer
	case "submitter":
		return aur.Submitter
	case "depends":
		return aur.Depends
	case "makedepends":
		return aur.MakeDepends
	case "optdepends":
		return aur.OptDepends
	case "checkdepends":
		return aur.CheckDepends
	case "provides":
		return aur.Provides
	case "conflicts":
		return aur.Conflicts
	case "replaces":
		return aur.Replaces
	case "groups":
		return aur.Groups
	case "keywords":
		return aur.Keywords
	case "comaintainers":
		return aur.CoMaintainers
	default:
		return aur.NameDesc
	}
}

// aurPkgSearchStringResolved renders a search result for an AUR package.
// It accepts pre-resolved installed state to avoid a second LocalPackage call.
func aurPkgSearchStringResolved(pkg *aur.Pkg, installed bool, localVersion string, singleLineResults bool) string {
	linkText := text.Bold(text.ColorHash("aur")) + "/" + text.Bold(pkg.Name)
	toPrint := text.CreateRepoLink("aur", "", pkg.Name, linkText) +
		" " + text.Cyan(pkg.Version) +
		text.Bold(" (+"+strconv.Itoa(pkg.NumVotes)) +
		" " + text.Bold(strconv.FormatFloat(pkg.Popularity, 'f', 2, 64)+") ")

	if ageTag := text.FormatAgeTag(int64(pkg.LastModified)); ageTag != "" {
		toPrint += ageTag + " "
	}

	if pkg.Maintainer == "" {
		toPrint += text.Bold(text.Red(gotext.Get("(Orphaned)"))) + " "
	}

	if pkg.OutOfDate != 0 {
		toPrint += text.Bold(text.Red(gotext.Get("(Out-of-date: %s)", text.FormatTime(pkg.OutOfDate)))) + " "
	}

	if installed {
		if localVersion != "" {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed: %s)", localVersion)))
		} else {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed)")))
		}
	}

	if singleLineResults {
		toPrint += "\t"
	} else {
		toPrint += "\n    "
	}

	toPrint += pkg.Description

	return toPrint
}

// syncPkgSearchStringResolved renders a search result for a sync package.
// It accepts pre-resolved groups and installed state to avoid second DB calls.
func syncPkgSearchStringResolved(pkg alpm.Package, groups []string, installed bool, localVersion string, singleLineResults bool) string {
	linkText := text.Bold(text.ColorHash(pkg.DB().Name())) + "/" + text.Bold(pkg.Name())
	toPrint := text.CreateRepoLink(pkg.DB().Name(), pkg.Architecture(), pkg.Name(), linkText) +
		" " + text.Cyan(pkg.Version()) +
		text.Bold(" ("+text.Human(pkg.Size())+
			" "+text.Human(pkg.ISize())+") ")

	if len(groups) != 0 {
		toPrint += fmt.Sprint(groups, " ")
	}

	if installed {
		if localVersion != "" {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed: %s)", localVersion)))
		} else {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed)")))
		}
	}

	if singleLineResults {
		toPrint += "\t"
	} else {
		toPrint += "\n    "
	}

	toPrint += pkg.Description()

	return toPrint
}
