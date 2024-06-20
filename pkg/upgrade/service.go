package upgrade

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/multierror"
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

// upGraph adds packages to upgrade to the graph.
func (u *UpgradeService) upGraph(ctx context.Context, graph *topo.Graph[string, *dep.InstallInfo],
	enableDowngrade bool,
	filter Filter,
) (err error) {
	var (
		develUp UpSlice
		errs    multierror.MultiError
		aurdata = make(map[string]*aur.Pkg)
		aurUp   UpSlice
	)

	remote := u.dbExecutor.InstalledRemotePackages()
	remoteNames := u.dbExecutor.InstalledRemotePackageNames()

	if u.cfg.Mode.AtLeastAUR() {
		u.log.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		_aurdata, err := u.aurCache.Get(ctx, &aur.Query{Needles: remoteNames, By: aur.Name})

		errs.Add(err)

		if err == nil {
			for i := range _aurdata {
				pkg := &_aurdata[i]
				aurdata[pkg.Name] = pkg
				u.AURWarnings.AddToWarnings(remote, pkg)
			}

			u.AURWarnings.CalculateMissing(remoteNames, remote, aurdata)

			aurUp = UpAUR(u.log, remote, aurdata, u.cfg.TimeUpdate, enableDowngrade)

			if u.cfg.Devel {
				u.log.OperationInfoln(gotext.Get("Checking development packages..."))

				develUp = UpDevel(ctx, u.log, remote, aurdata, u.vcsStore)

				u.vcsStore.CleanOrphans(remote)
			}
		}
	}

	aurPkgsAdded := []*aur.Pkg{}

	names := mapset.NewThreadUnsafeSet[string]()
	for i := range develUp.Up {
		up := &develUp.Up[i]
		// check if deps are satisfied for aur packages
		reason := dep.Explicit
		if up.Reason == alpm.PkgReasonDepend {
			reason = dep.Dep
		}

		if filter != nil && !filter(up) {
			continue
		}

		aurPkg := aurdata[up.Name]
		graph = u.grapher.GraphAURTarget(ctx, graph, aurPkg, &dep.InstallInfo{
			Reason:       reason,
			Source:       dep.AUR,
			AURBase:      &aurPkg.PackageBase,
			Upgrade:      true,
			Devel:        true,
			LocalVersion: up.LocalVersion,
			Version:      up.RemoteVersion,
		})
		names.Add(up.Name)
		aurPkgsAdded = append(aurPkgsAdded, aurPkg)
	}

	for i := range aurUp.Up {
		up := &aurUp.Up[i]
		// add devel packages if they are not already in the list
		if names.Contains(up.Name) {
			continue
		}

		// check if deps are satisfied for aur packages
		reason := dep.Explicit
		if up.Reason == alpm.PkgReasonDepend {
			reason = dep.Dep
		}

		if filter != nil && !filter(up) {
			continue
		}

		aurPkg := aurdata[up.Name]
		graph = u.grapher.GraphAURTarget(ctx, graph, aurPkg, &dep.InstallInfo{
			Reason:       reason,
			Source:       dep.AUR,
			AURBase:      &aurPkg.PackageBase,
			Upgrade:      true,
			Version:      up.RemoteVersion,
			LocalVersion: up.LocalVersion,
		})
		aurPkgsAdded = append(aurPkgsAdded, aurPkg)
	}

	u.grapher.AddDepsForPkgs(ctx, aurPkgsAdded, graph)

	if u.cfg.Mode.AtLeastRepo() {
		u.log.OperationInfoln(gotext.Get("Searching databases for updates..."))

		syncUpgrades, err := u.dbExecutor.SyncUpgrades(enableDowngrade)
		for _, up := range syncUpgrades {
			if filter != nil && !filter(&db.Upgrade{
				Name:          up.Package.Name(),
				RemoteVersion: up.Package.Version(),
				Repository:    up.Package.DB().Name(),
				Base:          up.Package.Base(),
				LocalVersion:  up.LocalVersion,
				Reason:        up.Reason,
			}) {
				continue
			}

			upgradeInfo := up
			graph = u.grapher.GraphSyncPkg(ctx, graph, up.Package, &upgradeInfo)
		}

		errs.Add(err)
	}

	return errs.Return()
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
			reducedParents := parents.Slice()[:min(cutOffExtra, len(parents))]
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
	if graph == nil {
		graph = dep.NewGraph()
	}

	err := u.upGraph(ctx, graph, enableDowngrade, filter)
	if err != nil {
		return graph, err
	}

	if graph.Len() == 0 {
		return graph, nil
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

	// true if user doesn't want to include specific repositories/packages
	noIncludes := len(include) == 0 && otherInclude.Cardinality() == 0

	excluded := make([]string, 0)
	for i := range allUp.Up {
		up := &allUp.Up[i]
		upgradeID := len(allUp.Up) - i

		// check if user wants to exclude specific things (true) or include specific things
		if noIncludes {
			// exclude repositories mentioned by the user
			if otherExclude.Contains(up.Repository) {
				u.log.Debugln("pruning", up.Name)
				excluded = append(excluded, graph.Prune(up.Name)...)
			}
			// exclude packages mentioned by the user
			if exclude.Get(upgradeID) {
				u.log.Debugln("pruning", up.Name)
				excluded = append(excluded, graph.Prune(up.Name)...)
			}

			// If the user explicitly wants to include a package/repository, exclude everything else
		} else if !include.Get(upgradeID) && !otherInclude.Contains(up.Repository) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
		}
	}

	return excluded, nil
}
