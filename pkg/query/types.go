package query

import (
	"fmt"
	"strconv"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/text"
)

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

func aurPkgSearchString(
	pkg *aur.Pkg,
	dbExecutor db.Executor,
	singleLineResults bool,
) string {
	toPrint := text.Bold(text.ColorHash("aur")) + "/" + text.Bold(pkg.Name) +
		" " + text.Cyan(pkg.Version) +
		text.Bold(" (+"+strconv.Itoa(pkg.NumVotes)) +
		" " + text.Bold(strconv.FormatFloat(pkg.Popularity, 'f', 2, 64)+") ")

	if pkg.Maintainer == "" {
		toPrint += text.Bold(text.Red(gotext.Get("(Orphaned)"))) + " "
	}

	if pkg.OutOfDate != 0 {
		toPrint += text.Bold(text.Red(gotext.Get("(Out-of-date: %s)", text.FormatTime(pkg.OutOfDate)))) + " "
	}

	if localPkg := dbExecutor.LocalPackage(pkg.Name); localPkg != nil {
		if localPkg.Version() != pkg.Version {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed: %s)", localPkg.Version())))
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

// PrintSearch receives a RepoSearch type and outputs pretty text.
func syncPkgSearchString(pkg alpm.IPackage, dbExecutor db.Executor, singleLineResults bool) string {
	toPrint := text.Bold(text.ColorHash(pkg.DB().Name())) + "/" + text.Bold(pkg.Name()) +
		" " + text.Cyan(pkg.Version()) +
		text.Bold(" ("+text.Human(pkg.Size())+
			" "+text.Human(pkg.ISize())+") ")

	packageGroups := dbExecutor.PackageGroups(pkg)
	if len(packageGroups) != 0 {
		toPrint += fmt.Sprint(packageGroups, " ")
	}

	if localPkg := dbExecutor.LocalPackage(pkg.Name()); localPkg != nil {
		if localPkg.Version() != pkg.Version() {
			toPrint += text.Bold(text.Green(gotext.Get("(Installed: %s)", localPkg.Version())))
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
