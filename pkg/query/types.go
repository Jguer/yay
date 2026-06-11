package query

import (
	"fmt"
	"strconv"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/text"
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

func aurPkgSearchString(
	pkg *aur.Pkg,
	dbExecutor db.Executor,
	singleLineResults bool,
) string {
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
func syncPkgSearchString(pkg alpm.Package, dbExecutor db.Executor, singleLineResults bool) string {
	linkText := text.Bold(text.ColorHash(pkg.DB().Name())) + "/" + text.Bold(pkg.Name())
	toPrint := text.CreateRepoLink(pkg.DB().Name(), pkg.Architecture(), pkg.Name(), linkText) +
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

// aurPkgToMap builds the Lua-facing table data for an AUR search result.
func aurPkgToMap(pkg *aur.Pkg, dbExecutor db.Executor) map[string]any {
	provides := make([]string, len(pkg.Provides))
	copy(provides, pkg.Provides)

	m := map[string]any{
		"source":       "aur",
		"name":         pkg.Name,
		"version":      pkg.Version,
		"description":  pkg.Description,
		"votes":        pkg.NumVotes,
		"popularity":   pkg.Popularity,
		"out_of_date":  pkg.OutOfDate,
		"package_base": pkg.PackageBase,
		"provides":     provides,
	}

	if pkg.Maintainer != "" {
		m["maintainer"] = pkg.Maintainer
	}

	if localPkg := dbExecutor.LocalPackage(pkg.Name); localPkg != nil {
		m["installed"], m["installed_version"] = true, localPkg.Version()
	} else {
		m["installed"] = false
	}

	return m
}

// syncPkgToMap builds the Lua-facing table data for a repo search result.
func syncPkgToMap(pkg alpm.Package, dbExecutor db.Executor) map[string]any {
	m := map[string]any{
		"source":         pkg.DB().Name(),
		"name":           pkg.Name(),
		"version":        pkg.Version(),
		"description":    pkg.Description(),
		"size":           pkg.Size(),
		"installed_size": pkg.ISize(),
		"groups":         dbExecutor.PackageGroups(pkg),
	}

	if localPkg := dbExecutor.LocalPackage(pkg.Name()); localPkg != nil {
		m["installed"], m["installed_version"] = true, localPkg.Version()
	} else {
		m["installed"] = false
	}

	return m
}
