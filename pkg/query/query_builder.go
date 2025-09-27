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

	"github.com/Jguer/yay/v12/pkg/customrepo"
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
	queryMap          map[string]map[string]any
	bottomUp          bool
	singleLineResults bool
	separateSources   bool

	aurClient      aur.QueryClient
	customRepoMgr  *customrepo.Manager
	logger         *text.Logger
}

func NewSourceQueryBuilder(
	aurClient aur.QueryClient,
	customRepoMgr *customrepo.Manager,
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
		customRepoMgr:     customRepoMgr,
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
	source      string
	name        string
	description string
	votes       int
	provides    []string
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
	case "name":
		cmpResult = !text.LessRunes([]rune(pkgA.name), []rune(pkgB.name))
		if a.separateSources {
			cmpSources := strings.Compare(pkgA.source, pkgB.source)
			if cmpSources != 0 {
				cmpResult = cmpSources > 0
			}
		}
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

	// Parse repository-specific search targets (e.g., "aur/mama", "github-test/mama")
	var aurTargets, repoTargets, customRepoTargets []string
	var generalTargets []string

	for _, target := range pkgS {
		s.logger.Debugln("Processing target:", target)
		if strings.Contains(target, "/") {
			parts := strings.SplitN(target, "/", 2)
			if len(parts) == 2 {
				repoName, pkgName := parts[0], parts[1]
				s.logger.Debugln("Split target into repo:", repoName, "package:", pkgName)
				
				// Check if it's a custom repository
				if s.customRepoMgr != nil {
					if _, exists := s.customRepoMgr.GetRepository(repoName); exists {
						s.logger.Debugln("Found custom repository:", repoName)
						customRepoTargets = append(customRepoTargets, pkgName)
						continue
					}
				}
				
				// Check if it's AUR
				if repoName == "aur" {
					s.logger.Debugln("Found AUR repository:", repoName)
					aurTargets = append(aurTargets, pkgName)
					continue
				}
				
				// Check if it's a known repository
				if s.isKnownRepo(repoName, dbExecutor) {
					s.logger.Debugln("Found known repository:", repoName)
					repoTargets = append(repoTargets, target)
					continue
				}
				
				s.logger.Debugln("Unknown repository:", repoName, "adding to general targets")
			}
		}
		
		// If no repository specified, add to general targets
		generalTargets = append(generalTargets, target)
	}

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

	// Search AUR packages
	if s.targetMode.AtLeastAUR() {
		searchTargets := generalTargets
		if len(aurTargets) > 0 {
			searchTargets = aurTargets
		}
		
		if len(searchTargets) > 0 {
			var aurResults []aur.Pkg
			aurResults, aurErr = queryAUR(ctx, s.aurClient, searchTargets, s.searchBy)
			dbName := sourceAUR

			for i := range aurResults {
				if s.queryMap[dbName] == nil {
					s.queryMap[dbName] = map[string]any{}
				}

				by := getSearchBy(s.searchBy)
				if (by == aur.NameDesc || by == aur.None || by == aur.Name) &&
					!matchesSearch(&aurResults[i], searchTargets) {
					continue
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
	}

	// Search repository packages
	var repoResults []alpm.IPackage
	if s.targetMode.AtLeastRepo() {
		searchTargets := generalTargets
		if len(repoTargets) > 0 {
			// Extract package names from repo/package format
			searchTargets = make([]string, len(repoTargets))
			for i, target := range repoTargets {
				if strings.Contains(target, "/") {
					parts := strings.SplitN(target, "/", 2)
					if len(parts) == 2 {
						searchTargets[i] = parts[1] // Use package name only
					} else {
						searchTargets[i] = target
					}
				} else {
					searchTargets[i] = target
				}
			}
		}
		
		if len(searchTargets) > 0 {
			repoResults = dbExecutor.SyncPackages(searchTargets...)

			for i := range repoResults {
				dbName := repoResults[i].DB().Name()
				if s.queryMap[dbName] == nil {
					s.queryMap[dbName] = map[string]any{}
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
	}

	// Search custom repositories
	if s.customRepoMgr != nil {
		searchTargets := generalTargets
		if len(customRepoTargets) > 0 {
			searchTargets = customRepoTargets
		}
		
		if len(searchTargets) > 0 {
			customResults, err := s.customRepoMgr.SearchAll(ctx, strings.Join(searchTargets, " "))
			if err != nil {
				s.logger.Warnln("Error searching custom repositories:", err)
			} else {
				for _, pkg := range customResults {
					dbName := pkg.Source
					if s.queryMap[dbName] == nil {
						s.queryMap[dbName] = map[string]any{}
					}

					s.queryMap[dbName][pkg.Name] = pkg

					sortableResults.results = append(sortableResults.results, abstractResult{
						source:      dbName,
						name:        pkg.Name,
						description: pkg.Description,
						provides:    pkg.Provides,
						votes:       -1, // Custom repos don't have votes
					})
				}
			}
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
		case customrepo.PackageInfo:
			toPrint += customRepoPkgSearchString(&pPkg, dbExecutor, s.singleLineResults)
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

// isKnownRepo checks if a repository name is a known pacman repository
func (s *SourceQueryBuilder) isKnownRepo(repoName string, dbExecutor db.Executor) bool {
	repos := dbExecutor.Repos()
	for _, repo := range repos {
		if repo == repoName {
			return true
		}
	}
	return false
}
