package query

import (
	"context"
	"io"
	"sort"
	"strings"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

type SearchVerbosity int

// Verbosity settings for search.
const (
	NumberMenu SearchVerbosity = iota
	Detailed
	Minimal
)

type SourceQueryBuilder struct {
	repoQuery
	aurQuery
	bottomUp          bool
	sortBy            string
	targetMode        parser.TargetMode
	searchBy          string
	singleLineResults bool
}

func NewSourceQueryBuilder(
	sortBy string,
	targetMode parser.TargetMode,
	searchBy string,
	bottomUp,
	singleLineResults bool,
) *SourceQueryBuilder {
	return &SourceQueryBuilder{
		repoQuery:         []alpm.IPackage{},
		aurQuery:          []aur.Pkg{},
		bottomUp:          bottomUp,
		sortBy:            sortBy,
		targetMode:        targetMode,
		searchBy:          searchBy,
		singleLineResults: singleLineResults,
	}
}

func (s *SourceQueryBuilder) Execute(ctx context.Context, dbExecutor db.Executor, aurClient *aur.Client, pkgS []string) {
	var aurErr error

	pkgS = RemoveInvalidTargets(pkgS, s.targetMode)

	if s.targetMode.AtLeastAUR() {
		s.aurQuery, aurErr = queryAUR(ctx, aurClient, pkgS, s.searchBy, s.bottomUp, s.sortBy)
	}

	if s.targetMode.AtLeastRepo() {
		s.repoQuery = queryRepo(pkgS, dbExecutor, s.bottomUp)
	}

	if aurErr != nil && len(s.repoQuery) != 0 {
		text.Errorln(ErrAURSearch{inner: aurErr})
		text.Warnln(gotext.Get("Showing repo packages only"))
	}
}

func (s *SourceQueryBuilder) Results(w io.Writer, dbExecutor db.Executor, verboseSearch SearchVerbosity) error {
	if s.aurQuery == nil || s.repoQuery == nil {
		return ErrNoQuery{}
	}

	if s.bottomUp {
		if s.targetMode.AtLeastAUR() {
			s.aurQuery.printSearch(w, len(s.repoQuery)+1, dbExecutor, verboseSearch, s.bottomUp, s.singleLineResults)
		}

		if s.targetMode.AtLeastRepo() {
			s.repoQuery.printSearch(w, dbExecutor, verboseSearch, s.bottomUp, s.singleLineResults)
		}
	} else {
		if s.targetMode.AtLeastRepo() {
			s.repoQuery.printSearch(w, dbExecutor, verboseSearch, s.bottomUp, s.singleLineResults)
		}

		if s.targetMode.AtLeastAUR() {
			s.aurQuery.printSearch(w, len(s.repoQuery)+1, dbExecutor, verboseSearch, s.bottomUp, s.singleLineResults)
		}
	}

	return nil
}

func (s *SourceQueryBuilder) Len() int {
	return len(s.repoQuery) + len(s.aurQuery)
}

func (s *SourceQueryBuilder) GetTargets(include, exclude intrange.IntRanges,
	otherExclude stringset.StringSet) ([]string, error) {
	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	var targets []string

	for i, pkg := range s.repoQuery {
		var target int

		if s.bottomUp {
			target = len(s.repoQuery) - i
		} else {
			target = i + 1
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			targets = append(targets, pkg.DB().Name()+"/"+pkg.Name())
		}
	}

	for i := range s.aurQuery {
		var target int

		if s.bottomUp {
			target = len(s.aurQuery) - i + len(s.repoQuery)
		} else {
			target = i + 1 + len(s.repoQuery)
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			targets = append(targets, "aur/"+s.aurQuery[i].Name)
		}
	}

	return targets, nil
}

// queryRepo handles repo searches. Creates a RepoSearch struct.
func queryRepo(pkgInputN []string, dbExecutor db.Executor, bottomUp bool) repoQuery {
	s := repoQuery(dbExecutor.SyncPackages(pkgInputN...))

	if bottomUp {
		s.Reverse()
	}

	return s
}

// queryAUR searches AUR and narrows based on subarguments.
func queryAUR(ctx context.Context, aurClient *aur.Client, pkgS []string, searchBy string, bottomUp bool, sortBy string) (aurQuery, error) {
	var (
		r         []aur.Pkg
		err       error
		usedIndex int
	)

	by := getSearchBy(searchBy)

	if len(pkgS) == 0 {
		return nil, nil
	}

	for i, word := range pkgS {
		r, err = aurClient.Search(ctx, word, by)
		if err == nil {
			usedIndex = i

			break
		}
	}

	if err != nil {
		return nil, err
	}

	if len(pkgS) == 1 {
		sort.Sort(aurSortable{
			aurQuery: r,
			sortBy:   sortBy,
			bottomUp: bottomUp,
		})

		return r, err
	}

	var (
		aq aurQuery
		n  int
	)

	for i := range r {
		match := true

		for j, pkgN := range pkgS {
			if usedIndex == j {
				continue
			}

			name := strings.ToLower(r[i].Name)
			desc := strings.ToLower(r[i].Description)
			targ := strings.ToLower(pkgN)

			if !(strings.Contains(name, targ) || strings.Contains(desc, targ)) {
				match = false

				break
			}
		}

		if match {
			n++

			aq = append(aq, r[i])
		}
	}

	sort.Sort(aurSortable{
		aurQuery: aq,
		sortBy:   sortBy,
		bottomUp: bottomUp,
	})

	return aq, err
}
