package upgrade

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

const cutOffExtra = 2

type UpgradeService struct {
	grapher    *dep.Grapher
	aurCache   aur.QueryClient
	dbExecutor db.Executor
	vcsStore   vcs.Store
	cfg        *settings.Configuration
	log        *text.Logger
	noConfirm  bool

	AURWarnings *query.AURWarnings
}

func NewUpgradeService(grapher *dep.Grapher, aurCache aur.QueryClient,
	dbExecutor db.Executor, vcsStore vcs.Store,
	cfg *settings.Configuration, noConfirm bool, logger *text.Logger,
) *UpgradeService {
	return &UpgradeService{
		grapher:     grapher,
		aurCache:    aurCache,
		dbExecutor:  dbExecutor,
		vcsStore:    vcsStore,
		cfg:         cfg,
		noConfirm:   noConfirm,
		log:         logger,
		AURWarnings: query.NewWarnings(logger.Child("warnings")),
	}
}

func (u *UpgradeService) graphToUpSlice(graph *topo.Graph[string, *dep.InstallInfo]) (aurUp, repoUp UpSlice) {
	aurUp = UpSlice{Up: make([]Upgrade, 0, graph.Len())}
	repoUp = UpSlice{Up: make([]Upgrade, 0, graph.Len()), Repos: u.dbExecutor.Repos()}

	_ = graph.ForEach(func(name string, info *dep.InstallInfo) error {
		alpmReason := alpm.PkgReasonDepend
		if info.Reason == dep.Explicit {
			alpmReason = alpm.PkgReasonExplicit
		}

		parents := graph.ImmediateDependencies(name)
		extra := ""
		if len(parents) > 0 && !info.Upgrade && info.Reason == dep.MakeDep {
			reducedParents := parents.Slice()[:int(math.Min(cutOffExtra, float64(len(parents))))]
			if len(parents) > cutOffExtra {
				reducedParents = append(reducedParents, "...")
			}
			extra = fmt.Sprintf(" (%s of %s)", dep.ReasonNames[info.Reason], strings.Join(reducedParents, ", "))
		}

		if info.Source == dep.AUR {
			aurRepo := "aur"
			if info.Devel {
				aurRepo = "devel"
			}
			aurUp.Up = append(aurUp.Up, Upgrade{
				Name:          name,
				RemoteVersion: info.Version,
				Repository:    aurRepo,
				Base:          *info.AURBase,
				LocalVersion:  info.LocalVersion,
				Reason:        alpmReason,
				Extra:         extra,
			})
		} else if info.Source == dep.Sync {
			repoUp.Up = append(repoUp.Up, Upgrade{
				Name:          name,
				RemoteVersion: info.Version,
				Repository:    *info.SyncDBName,
				Base:          "",
				LocalVersion:  info.LocalVersion,
				Reason:        alpmReason,
				Extra:         extra,
			})
		}
		return nil
	})

	return aurUp, repoUp
}

func (u *UpgradeService) GraphUpgrades(ctx context.Context,
	graph *topo.Graph[string, *dep.InstallInfo],
	enableDowngrade bool, filter Filter,
) (*topo.Graph[string, *dep.InstallInfo], error) {
	graph, err := u.grapher.GraphUpgrades(ctx, graph, enableDowngrade)
	if err != nil {
		return graph, err
	}

	return graph, nil
}

// userExcludeUpgrades asks the user which packages to exclude from the upgrade and
// removes them from the graph
func (u *UpgradeService) UserExcludeUpgrades(graph *topo.Graph[string, *dep.InstallInfo]) ([]string, error) {
	if graph.Len() == 0 {
		return []string{}, nil
	}
	aurUp, repoUp := u.graphToUpSlice(graph)

	sort.Sort(repoUp)
	sort.Sort(aurUp)

	allUp := UpSlice{Repos: append(repoUp.Repos, aurUp.Repos...)}
	for _, up := range repoUp.Up {
		if up.LocalVersion == "" && up.Reason != alpm.PkgReasonExplicit {
			allUp.PulledDeps = append(allUp.PulledDeps, up)
		} else {
			allUp.Up = append(allUp.Up, up)
		}
	}

	for _, up := range aurUp.Up {
		if up.LocalVersion == "" && up.Reason != alpm.PkgReasonExplicit {
			allUp.PulledDeps = append(allUp.PulledDeps, up)
		} else {
			allUp.Up = append(allUp.Up, up)
		}
	}

	if len(allUp.PulledDeps) > 0 {
		u.log.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")),
			len(allUp.PulledDeps), text.Bold(gotext.Get("%s will also be installed for this operation.",
				gotext.GetN("dependency", "dependencies", len(allUp.PulledDeps)))))
		allUp.PrintDeps(u.log)
	}

	u.log.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")),
		len(allUp.Up), text.Bold(gotext.Get("%s to upgrade/install.", gotext.GetN("package", "packages", len(allUp.Up)))))
	allUp.Print(u.log)

	u.log.Infoln(gotext.Get("Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)"))
	u.log.Warnln(gotext.Get("Excluding packages may cause partial upgrades and break systems"))

	numbers, err := u.log.GetInput(u.cfg.AnswerUpgrade, settings.NoConfirm)
	if err != nil {
		return nil, err
	}

	// upgrade menu asks you which packages to NOT upgrade so in this case
	// exclude and include are kind of swapped
	exclude, include, otherExclude, otherInclude := intrange.ParseNumberMenu(numbers)
	isInclude := len(include) == 0 && otherInclude.Cardinality() == 0

	excluded := make([]string, 0)
	for i := range allUp.Up {
		up := &allUp.Up[i]

		if isInclude && otherExclude.Contains(up.Repository) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
			continue
		}

		if isInclude && exclude.Get(len(allUp.Up)-i) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
			continue
		}

		if !isInclude && !(include.Get(len(allUp.Up)-i) || otherInclude.Contains(up.Repository)) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
			continue
		}
	}

	return excluded, nil
}
