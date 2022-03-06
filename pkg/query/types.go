package query

import (
	"fmt"
	"io"
	"strconv"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/text"
)

type (
	aurQuery  []aur.Pkg       // Query is a collection of Results.
	repoQuery []alpm.IPackage // Query holds the results of a repository search.
)

type aurSortable struct {
	aurQuery
	sortBy   string
	bottomUp bool
}

func (r repoQuery) Reverse() {
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
}

func (r repoQuery) Less(i, j int) bool {
	return text.LessRunes([]rune(r[i].Name()), []rune(r[j].Name()))
}

func (q aurSortable) Len() int {
	return len(q.aurQuery)
}

func (q aurSortable) Less(i, j int) bool {
	var result bool

	switch q.sortBy {
	case "votes":
		result = q.aurQuery[i].NumVotes > q.aurQuery[j].NumVotes
	case "popularity":
		result = q.aurQuery[i].Popularity > q.aurQuery[j].Popularity
	case "name":
		result = text.LessRunes([]rune(q.aurQuery[i].Name), []rune(q.aurQuery[j].Name))
	case "base":
		result = text.LessRunes([]rune(q.aurQuery[i].PackageBase), []rune(q.aurQuery[j].PackageBase))
	case "submitted":
		result = q.aurQuery[i].FirstSubmitted < q.aurQuery[j].FirstSubmitted
	case "modified":
		result = q.aurQuery[i].LastModified < q.aurQuery[j].LastModified
	case "id":
		result = q.aurQuery[i].ID < q.aurQuery[j].ID
	case "baseid":
		result = q.aurQuery[i].PackageBaseID < q.aurQuery[j].PackageBaseID
	}

	if q.bottomUp {
		return !result
	}

	return result
}

func (q aurSortable) Swap(i, j int) {
	q.aurQuery[i], q.aurQuery[j] = q.aurQuery[j], q.aurQuery[i]
}

func getSearchBy(value string) aur.By {
	switch value {
	case "name":
		return aur.Name
	case "maintainer":
		return aur.Maintainer
	case "depends":
		return aur.Depends
	case "makedepends":
		return aur.MakeDepends
	case "optdepends":
		return aur.OptDepends
	case "checkdepends":
		return aur.CheckDepends
	default:
		return aur.NameDesc
	}
}

// PrintSearch handles printing search results in a given format.
func (q aurQuery) printSearch(
	w io.Writer,
	start int,
	dbExecutor db.Executor,
	searchMode SearchVerbosity,
	bottomUp,
	singleLineResults bool,
) {
	for i := range q {
		if searchMode == Minimal {
			_, _ = fmt.Fprintln(w, q[i].Name)
			continue
		}

		var toprint string

		if searchMode == NumberMenu {
			if bottomUp {
				toprint += text.Magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			} else {
				toprint += text.Magenta(strconv.Itoa(start+i) + " ")
			}
		}

		toprint += aurPkgSearchString(&q[i], dbExecutor, singleLineResults)
		_, _ = fmt.Fprintln(w, toprint)
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
func (r repoQuery) printSearch(w io.Writer, dbExecutor db.Executor, searchMode SearchVerbosity, bottomUp, singleLineResults bool) {
	for i, res := range r {
		if searchMode == Minimal {
			_, _ = fmt.Fprintln(w, res.Name())
			continue
		}

		var toprint string

		if searchMode == NumberMenu {
			if bottomUp {
				toprint += text.Magenta(strconv.Itoa(len(r)-i) + " ")
			} else {
				toprint += text.Magenta(strconv.Itoa(i+1) + " ")
			}
		}

		toprint += syncPkgSearchString(res, dbExecutor, singleLineResults)
		_, _ = fmt.Fprintln(w, toprint)
	}
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
