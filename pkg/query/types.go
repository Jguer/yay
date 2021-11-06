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

		toprint += text.Bold(text.ColorHash("aur")) + "/" + text.Bold(q[i].Name) +
			" " + text.Cyan(q[i].Version) +
			text.Bold(" (+"+strconv.Itoa(q[i].NumVotes)) +
			" " + text.Bold(strconv.FormatFloat(q[i].Popularity, 'f', 2, 64)+") ")

		if q[i].Maintainer == "" {
			toprint += text.Bold(text.Red(gotext.Get("(Orphaned)"))) + " "
		}

		if q[i].OutOfDate != 0 {
			toprint += text.Bold(text.Red(gotext.Get("(Out-of-date: %s)", text.FormatTime(q[i].OutOfDate)))) + " "
		}

		if pkg := dbExecutor.LocalPackage(q[i].Name); pkg != nil {
			if pkg.Version() != q[i].Version {
				toprint += text.Bold(text.Green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += text.Bold(text.Green(gotext.Get("(Installed)")))
			}
		}

		if singleLineResults {
			toprint += "\t"
		} else {
			toprint += "\n    "
		}

		toprint += q[i].Description
		_, _ = fmt.Fprintln(w, toprint)
	}
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

		toprint += text.Bold(text.ColorHash(res.DB().Name())) + "/" + text.Bold(res.Name()) +
			" " + text.Cyan(res.Version()) +
			text.Bold(" ("+text.Human(res.Size())+
				" "+text.Human(res.ISize())+") ")

		packageGroups := dbExecutor.PackageGroups(res)
		if len(packageGroups) != 0 {
			toprint += fmt.Sprint(packageGroups, " ")
		}

		if pkg := dbExecutor.LocalPackage(res.Name()); pkg != nil {
			if pkg.Version() != res.Version() {
				toprint += text.Bold(text.Green(gotext.Get("(Installed: %s)", pkg.Version())))
			} else {
				toprint += text.Bold(text.Green(gotext.Get("(Installed)")))
			}
		}

		if singleLineResults {
			toprint += "\t"
		} else {
			toprint += "\n    "
		}

		toprint += res.Description()

		_, _ = fmt.Fprintln(w, toprint)
	}
}
