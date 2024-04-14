package query

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

const sourceAUR = "aur"

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

type SourceQueryBuilder struct {
	results           []abstractResult
	sortBy            string
	searchBy          string
	targetMode        parser.TargetMode
	queryMap          map[string]map[string]interface{}
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
		queryMap:          map[string]map[string]interface{}{},
		results:           make([]abstractResult, 0, 100),
	}
}

type abstractResult struct {
	source      string
	id          int
	base        string
	baseid      int
	name        string
	description string
	votes       int
	provides    []string
	submitted   int
	modified    int
	popularity  float64
}

type abstractResults struct {
	results         []abstractResult
	search          string
	bottomUp        bool
	metric          strutil.StringMetric
	separateSources bool
	sortBy          string

	distanceCache       map[string]float64
	separateSourceCache map[string]float64
}

func (a *abstractResults) Len() int      { return len(a.results) }
func (a *abstractResults) Swap(i, j int) { a.results[i], a.results[j] = a.results[j], a.results[i] }

func (a *abstractResults) Less(i, j int) bool {
	pkgA := a.results[i]
	pkgB := a.results[j]

	var cmpResult bool

	switch a.sortBy {
	case "id":
		cmpResult = pkgA.id > pkgB.id
	case "base":
		cmpResult = !text.LessRunes([]rune(pkgA.base), []rune(pkgB.base))
		if a.separateSources {
			cmpSources := strings.Compare(pkgA.source, pkgB.source)
			if cmpSources != 0 {
				cmpResult = cmpSources > 0
			}
		}

	case "baseid":
		cmpResult = pkgA.baseid > pkgB.baseid
	case "name":
		cmpResult = !text.LessRunes([]rune(pkgA.name), []rune(pkgB.name))
		if a.separateSources {
			cmpSources := strings.Compare(pkgA.source, pkgB.source)
			if cmpSources != 0 {
				cmpResult = cmpSources > 0
			}
		}
	case "votes":
		cmpResult = pkgA.votes > pkgB.votes
	case "submitted":
		cmpResult = pkgA.submitted > pkgB.submitted
	case "modified":
		cmpResult = pkgA.modified > pkgB.modified
	case "popularity":
		cmpResult = pkgA.popularity > pkgB.popularity

	default:
		simA := a.calculateMetric(&pkgA)
		simB := a.calculateMetric(&pkgB)
		cmpResult = simA > simB
	}

	if a.bottomUp {
		cmpResult = !cmpResult
	}

	return cmpResult
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
		bottomUp:            s.bottomUp,
		metric:              metric,
		separateSources:     s.separateSources,
		sortBy:              s.sortBy,
		distanceCache:       map[string]float64{},
		separateSourceCache: map[string]float64{},
	}

	if s.targetMode.AtLeastAUR() {
		var aurResults []aur.Pkg
		aurResults, aurErr = queryAUR(ctx, s.aurClient, pkgS, s.searchBy)
		dbName := sourceAUR

		for i := range aurResults {
			if s.queryMap[dbName] == nil {
				s.queryMap[dbName] = map[string]interface{}{}
			}

			by := getSearchBy(s.searchBy)
			if (by == aur.NameDesc || by == aur.None || by == aur.Name) &&
				!matchesSearch(&aurResults[i], pkgS) {
				continue
			}

			s.queryMap[dbName][aurResults[i].Name] = aurResults[i]

			sortableResults.results = append(sortableResults.results, abstractResult{
				source:      dbName,
				id:          aurResults[i].ID,
				base:        aurResults[i].PackageBase,
				baseid:      aurResults[i].PackageBaseID,
				name:        aurResults[i].Name,
				description: aurResults[i].Description,
				provides:    aurResults[i].Provides,
				votes:       aurResults[i].NumVotes,
				submitted:   aurResults[i].FirstSubmitted,
				modified:    aurResults[i].LastModified,
				popularity:  aurResults[i].Popularity,
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
		case alpm.IPackage:
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

		if !(strings.Contains(name, targ) || strings.Contains(desc, targ)) {
			return false
		}
	}

	return true
}
