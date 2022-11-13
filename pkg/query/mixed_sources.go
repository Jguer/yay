package query

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

const sourceAUR = "aur"

type Builder interface {
	Len() int
	Execute(ctx context.Context, dbExecutor db.Executor, pkgS []string)
	Results(w io.Writer, dbExecutor db.Executor, verboseSearch SearchVerbosity) error
	GetTargets(include, exclude intrange.IntRanges, otherExclude stringset.StringSet) ([]string, error)
}

type MixedSourceQueryBuilder struct {
	results           []abstractResult
	sortBy            string
	searchBy          string
	targetMode        parser.TargetMode
	queryMap          map[string]map[string]interface{}
	bottomUp          bool
	singleLineResults bool

	aurClient aur.ClientInterface
}

func NewMixedSourceQueryBuilder(
	aurClient aur.ClientInterface,
	sortBy string,
	targetMode parser.TargetMode,
	searchBy string,
	bottomUp,
	singleLineResults bool,
) *MixedSourceQueryBuilder {
	return &MixedSourceQueryBuilder{
		aurClient:         aurClient,
		bottomUp:          bottomUp,
		sortBy:            sortBy,
		targetMode:        targetMode,
		searchBy:          searchBy,
		singleLineResults: singleLineResults,
		queryMap:          map[string]map[string]interface{}{},
		results:           make([]abstractResult, 0, 100),
	}
}

type abstractResult struct {
	source      string
	name        string
	description string
	votes       int
	provides    []string
}

type abstractResults struct {
	results       []abstractResult
	search        string
	distanceCache map[string]float64
	bottomUp      bool
	metric        strutil.StringMetric
}

func (a *abstractResults) Len() int      { return len(a.results) }
func (a *abstractResults) Swap(i, j int) { a.results[i], a.results[j] = a.results[j], a.results[i] }

func (a *abstractResults) GetMetric(pkg *abstractResult) float64 {
	if v, ok := a.distanceCache[pkg.name]; ok {
		return v
	}

	sim := strutil.Similarity(pkg.name, a.search, a.metric)

	for _, prov := range pkg.provides {
		// If the package provides search, it's a perfect match
		// AUR packages don't populate provides
		candidate := strutil.Similarity(prov, a.search, a.metric)
		if candidate > sim {
			sim = candidate
		}
	}

	simDesc := strutil.Similarity(pkg.description, a.search, a.metric)

	// slightly overweight sync sources by always giving them max popularity
	popularity := 1.0
	if pkg.source == sourceAUR {
		popularity = float64(pkg.votes) / float64(pkg.votes+60)
	}

	sim = sim*0.6 + simDesc*0.2 + popularity*0.2

	a.distanceCache[pkg.name] = sim

	return sim
}

func (a *abstractResults) Less(i, j int) bool {
	pkgA := a.results[i]
	pkgB := a.results[j]

	simA := a.GetMetric(&pkgA)
	simB := a.GetMetric(&pkgB)

	if a.bottomUp {
		return simA < simB
	}

	return simA > simB
}

func (s *MixedSourceQueryBuilder) Execute(ctx context.Context, dbExecutor db.Executor, pkgS []string) {
	var aurErr error

	pkgS = RemoveInvalidTargets(pkgS, s.targetMode)

	metric := &metrics.JaroWinkler{
		CaseSensitive: false,
	}

	sortableResults := &abstractResults{
		results:       []abstractResult{},
		search:        strings.Join(pkgS, ""),
		distanceCache: map[string]float64{},
		bottomUp:      s.bottomUp,
		metric:        metric,
	}

	if s.targetMode.AtLeastAUR() {
		var aurResults aurQuery
		aurResults, aurErr = queryAUR(ctx, s.aurClient, pkgS, s.searchBy)
		dbName := sourceAUR

		for i := range aurResults {
			if s.queryMap[dbName] == nil {
				s.queryMap[dbName] = map[string]interface{}{}
			}

			s.queryMap[dbName][aurResults[i].Name] = aurResults[i]

			sortableResults.results = append(sortableResults.results, abstractResult{
				source:      dbName,
				name:        aurResults[i].Name,
				description: aurResults[i].Description,
				provides:    aurResults[i].Provides,
				votes:       aurResults[i].NumVotes,
			})
		}
	}

	var repoResults []alpm.IPackage
	if s.targetMode.AtLeastRepo() {
		repoResults = dbExecutor.SyncPackages(pkgS...)

		for i := range repoResults {
			dbName := repoResults[i].DB().Name()
			if s.queryMap[dbName] == nil {
				s.queryMap[dbName] = map[string]interface{}{}
			}

			s.queryMap[dbName][repoResults[i].Name()] = repoResults[i]

			rawProvides := repoResults[i].Provides().Slice()

			provides := make([]string, len(rawProvides))
			for j := range rawProvides {
				provides[j] = rawProvides[j].Name
			}

			sortableResults.results = append(sortableResults.results, abstractResult{
				source:      repoResults[i].DB().Name(),
				name:        repoResults[i].Name(),
				description: repoResults[i].Description(),
				provides:    provides,
				votes:       -1,
			})
		}
	}

	sort.Sort(sortableResults)
	s.results = sortableResults.results

	if aurErr != nil {
		text.Errorln(ErrAURSearch{inner: aurErr})

		if len(repoResults) != 0 {
			text.Warnln(gotext.Get("Showing repo packages only"))
		}
	}
}

func (s *MixedSourceQueryBuilder) Results(w io.Writer, dbExecutor db.Executor, verboseSearch SearchVerbosity) error {
	for i := range s.results {
		if verboseSearch == Minimal {
			_, _ = fmt.Fprintln(w, s.results[i].name)
			continue
		}

		var toPrint string

		if verboseSearch == NumberMenu {
			if s.bottomUp {
				toPrint += text.Magenta(strconv.Itoa(len(s.results)-i)) + " "
			} else {
				toPrint += text.Magenta(strconv.Itoa(i+1)) + " "
			}
		}

		pkg := s.queryMap[s.results[i].source][s.results[i].name]
		if s.results[i].source == sourceAUR {
			aurPkg := pkg.(aur.Pkg)
			toPrint += aurPkgSearchString(&aurPkg, dbExecutor, s.singleLineResults)
		} else {
			syncPkg := pkg.(alpm.IPackage)
			toPrint += syncPkgSearchString(syncPkg, dbExecutor, s.singleLineResults)
		}

		fmt.Fprintln(w, toPrint)
	}

	return nil
}

func (s *MixedSourceQueryBuilder) Len() int {
	return len(s.results)
}

func (s *MixedSourceQueryBuilder) GetTargets(include, exclude intrange.IntRanges,
	otherExclude stringset.StringSet,
) ([]string, error) {
	var (
		isInclude = len(exclude) == 0 && len(otherExclude) == 0
		targets   []string
		lenRes    = len(s.results)
	)

	for i := 0; i <= s.Len(); i++ {
		target := i - 1
		if s.bottomUp {
			target = lenRes - i
		}

		if (isInclude && include.Get(i)) || (!isInclude && !exclude.Get(i)) {
			targets = append(targets, s.results[target].source+"/"+s.results[target].name)
		}
	}

	return targets, nil
}
