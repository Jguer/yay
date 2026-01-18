package query

import (
	"cmp"
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

type SearchVerbosity int

// Verbosity settings for search.
const (
	NumberMenu SearchVerbosity = iota
	Detailed
	Minimal
)

type Builder interface {
	Len() int
	Execute(ctx context.Context, dbExecutor db.Executor, pkgS []string)
	Results(dbExecutor db.Executor, verboseSearch SearchVerbosity) error
	GetTargets(include, exclude intrange.IntRanges, otherExclude mapset.Set[string]) ([]string, error)
}

type SortFunc func(pkgA, pkgB abstractResult) int

type SourceQueryBuilder struct {
	results           []abstractResult
	sortBy            string
	searchBy          string
	targetMode        parser.TargetMode
	queryMap          map[string]map[string]any
	bottomUp          bool
	singleLineResults bool
	separateSources   bool

	aurClient aur.QueryClient
	logger    *text.Logger
}

func NewSourceQueryBuilder(
	aurClient aur.QueryClient,
	logger *text.Logger,
	sortBy string,
	targetMode parser.TargetMode,
	searchBy string,
	bottomUp,
	singleLineResults bool,
	separateSources bool,
) *SourceQueryBuilder {
	return &SourceQueryBuilder{
		aurClient:         aurClient,
		logger:            logger,
		bottomUp:          bottomUp,
		sortBy:            sortBy,
		targetMode:        targetMode,
		searchBy:          searchBy,
		singleLineResults: singleLineResults,
		separateSources:   separateSources,
		queryMap:          map[string]map[string]any{},
		results:           make([]abstractResult, 0, 100),
	}
}

type abstractResult struct {
	source         string
	name           string
	description    string
	packageBase    string
	votes          int
	popularity     float64
	firstSubmitted int
	lastModified   int
	provides       []string
}

type abstractResults struct {
	results         []abstractResult
	search          string
	metric          strutil.StringMetric
	separateSources bool
	sortByFunc      SortFunc
	repoOrder       []string

	distanceCache       map[string]float64
	separateSourceCache map[string]float64
}

func (a *abstractResults) Len() int      { return len(a.results) }
func (a *abstractResults) Swap(i, j int) { a.results[i], a.results[j] = a.results[j], a.results[i] }

func (a *abstractResults) Less(i, j int) bool {
	pkgA := a.results[i]
	pkgB := a.results[j]
	// Sort in descending order by default
	return a.sortByFunc(pkgA, pkgB) > 0
}

func (a *abstractResults) GetSortFunc(sortBy string, bottomUp bool) SortFunc {
	var sortFunc SortFunc

	// Primary sort
	switch sortBy {
	case "base":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.packageBase, pkgB.packageBase)
		}
	case "modified":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.lastModified, pkgB.lastModified)
		}
	case "name":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.name, pkgB.name)
		}
	case "popularity":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.popularity, pkgB.popularity)
		}
	case "submitted":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.firstSubmitted, pkgB.firstSubmitted)
		}
	case "votes":
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return cmp.Compare(pkgA.votes, pkgB.votes)
		}
	default:
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return 0
		}
	}

	// Sort by metric as a tie-breaker. Also handle separating sources when not a tie
	{
		originalSortFunc := sortFunc
		sortFunc = func(pkgA, pkgB abstractResult) int {
			if cmpResult := originalSortFunc(pkgA, pkgB); cmpResult != 0 {
				if a.separateSources {
					if cmpSources := strings.Compare(pkgA.source, pkgB.source); cmpSources != 0 {
						return cmpSources
					}
				}
				return cmpResult
			}

			metricA := a.calculateMetric(&pkgA)
			metricB := a.calculateMetric(&pkgB)
			return cmp.Compare(metricA, metricB)
		}
	}

	if bottomUp {
		// Invert sort for bottom-up sorting
		originalSortFunc := sortFunc
		sortFunc = func(pkgA, pkgB abstractResult) int {
			return -originalSortFunc(pkgA, pkgB)
		}
	}

	return sortFunc
}

func (s *SourceQueryBuilder) Execute(ctx context.Context, dbExecutor db.Executor, pkgS []string) {
	var aurErr error

	pkgS = RemoveInvalidTargets(s.logger, pkgS, s.targetMode)

	metric := &metrics.Hamming{
		CaseSensitive: false,
	}

	sortableResults := &abstractResults{
		results:             []abstractResult{},
		search:              strings.Join(pkgS, ""),
		metric:              metric,
		separateSources:     s.separateSources,
		repoOrder:           dbExecutor.Repos(),
		distanceCache:       map[string]float64{},
		separateSourceCache: map[string]float64{},
	}
	sortableResults.sortByFunc = sortableResults.GetSortFunc(s.sortBy, s.bottomUp)

	var repoResults []alpm.Package
	if s.targetMode.AtLeastRepo() {
		repoResults = dbExecutor.SyncPackages(pkgS...)

		for i := range repoResults {
			dbName := repoResults[i].DB().Name()
			if s.queryMap[dbName] == nil {
				s.queryMap[dbName] = map[string]any{}
			}

			s.queryMap[dbName][repoResults[i].Name()] = repoResults[i]

			rawProvides := repoResults[i].Provides()

			provides := make([]string, len(rawProvides))
			for j := range rawProvides {
				provides[j] = rawProvides[j].Name
			}

			sortableResults.results = append(sortableResults.results, abstractResult{
				source:         repoResults[i].DB().Name(),
				name:           repoResults[i].Name(),
				description:    repoResults[i].Description(),
				packageBase:    repoResults[i].Base(),
				votes:          -1,
				popularity:     -1,
				firstSubmitted: -1,
				lastModified:   -1,
				provides:       provides,
			})
		}
	}

	if s.targetMode.AtLeastAUR() {
		var aurResults []aur.Pkg
		aurResults, aurErr = queryAUR(ctx, s.aurClient, pkgS, s.searchBy)
		dbName := "aur"

		for i := range aurResults {
			if s.queryMap[dbName] == nil {
				s.queryMap[dbName] = map[string]any{}
			}

			by := getSearchBy(s.searchBy)
			if (by == aur.NameDesc || by == aur.None || by == aur.Name) &&
				!matchesSearch(&aurResults[i], pkgS) {
				continue
			}

			s.queryMap[dbName][aurResults[i].Name] = aurResults[i]

			sortableResults.results = append(sortableResults.results, abstractResult{
				source:         dbName,
				name:           aurResults[i].Name,
				description:    aurResults[i].Description,
				packageBase:    aurResults[i].PackageBase,
				votes:          aurResults[i].NumVotes,
				popularity:     aurResults[i].Popularity,
				firstSubmitted: aurResults[i].FirstSubmitted,
				lastModified:   aurResults[i].LastModified,
				provides:       aurResults[i].Provides,
			})
		}
	}

	sort.Sort(sortableResults)
	s.results = sortableResults.results

	if aurErr != nil {
		s.logger.Errorln(ErrAURSearch{inner: aurErr})

		if len(repoResults) != 0 {
			s.logger.Warnln(gotext.Get("Showing repo packages only"))
		}
	}
}

func (s *SourceQueryBuilder) Results(dbExecutor db.Executor, verboseSearch SearchVerbosity) error {
	for i := range s.results {
		if verboseSearch == Minimal {
			s.logger.Println(s.results[i].name)
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

		switch pPkg := pkg.(type) {
		case aur.Pkg:
			toPrint += aurPkgSearchString(&pPkg, dbExecutor, s.singleLineResults)
		case alpm.Package:
			toPrint += syncPkgSearchString(pPkg, dbExecutor, s.singleLineResults)
		}

		s.logger.Println(toPrint)
	}

	return nil
}

func (s *SourceQueryBuilder) Len() int {
	return len(s.results)
}

func (s *SourceQueryBuilder) GetTargets(include, exclude intrange.IntRanges,
	otherExclude mapset.Set[string],
) ([]string, error) {
	var (
		isInclude = len(exclude) == 0 && otherExclude.Cardinality() == 0
		targets   []string
		lenRes    = len(s.results)
	)

	for i := 1; i <= s.Len(); i++ {
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

func matchesSearch(pkg *aur.Pkg, terms []string) bool {
	if len(terms) <= 1 {
		return true
	}

	for _, pkgN := range terms {
		if strings.IndexFunc(pkgN, unicode.IsSymbol) != -1 {
			return true
		}

		name := strings.ToLower(pkg.Name)
		desc := strings.ToLower(pkg.Description)
		targ := strings.ToLower(pkgN)

		if !strings.Contains(name, targ) && !strings.Contains(desc, targ) {
			return false
		}
	}

	return true
}
