package upgrade

import (
	"context"
	"sort"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/topo"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

type UpgradeService struct {
	grapher    *dep.Grapher
	aurCache   aur.QueryClient
	dbExecutor db.Executor
	vcsStore   vcs.Store
	runtime    *settings.Runtime
	cfg        *settings.Configuration
	log        *text.Logger
	noConfirm  bool
}

func NewUpgradeService(grapher *dep.Grapher, aurCache aur.QueryClient,
	dbExecutor db.Executor, vcsStore vcs.Store, runtime *settings.Runtime,
	cfg *settings.Configuration, noConfirm bool, logger *text.Logger,
) *UpgradeService {
	return &UpgradeService{
		grapher:    grapher,
		aurCache:   aurCache,
		dbExecutor: dbExecutor,
		vcsStore:   vcsStore,
		runtime:    runtime,
		cfg:        cfg,
		noConfirm:  noConfirm,
		log:        logger,
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

	if u.runtime.Mode.AtLeastAUR() {
		u.log.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		_aurdata, err := u.aurCache.Get(ctx, &aur.Query{Needles: remoteNames, By: aur.Name})

		errs.Add(err)

		if err == nil {
			for i := range _aurdata {
				pkg := &_aurdata[i]
				aurdata[pkg.Name] = pkg
			}

			aurUp = UpAUR(u.log, remote, aurdata, u.cfg.TimeUpdate, enableDowngrade)
		}

		if u.cfg.Devel {
			u.log.OperationInfoln(gotext.Get("Checking development packages..."))

			develUp = UpDevel(ctx, u.log, remote, aurdata, u.vcsStore)

			u.vcsStore.CleanOrphans(remote)
		}
	}

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
	}

	if u.cfg.Runtime.Mode.AtLeastRepo() {
		u.log.OperationInfoln(gotext.Get("Searching databases for updates..."))

		syncUpgrades, err := u.dbExecutor.SyncUpgrades(enableDowngrade)
		for _, up := range syncUpgrades {
			dbName := up.Package.DB().Name()
			if filter != nil && !filter(&db.Upgrade{
				Name:          up.Package.Name(),
				RemoteVersion: up.Package.Version(),
				Repository:    dbName,
				Base:          up.Package.Base(),
				LocalVersion:  up.LocalVersion,
				Reason:        up.Reason,
			}) {
				continue
			}

			reason := dep.Explicit
			if up.Reason == alpm.PkgReasonDepend {
				reason = dep.Dep
			}

			graph = u.grapher.GraphSyncPkg(ctx, graph, up.Package, &dep.InstallInfo{
				Source:       dep.Sync,
				Reason:       reason,
				Version:      up.Package.Version(),
				SyncDBName:   &dbName,
				LocalVersion: up.LocalVersion,
				Upgrade:      true,
			})
		}

		errs.Add(err)
	}

	return errs.Return()
}

func (u *UpgradeService) graphToUpSlice(graph *topo.Graph[string, *dep.InstallInfo]) (aurUp, repoUp UpSlice) {
	aurUp = UpSlice{Up: make([]Upgrade, 0, graph.Len())}
	repoUp = UpSlice{Up: make([]Upgrade, 0, graph.Len()), Repos: u.dbExecutor.Repos()}

	_ = graph.ForEach(func(name string, info *dep.InstallInfo) error {
		alpmReason := alpm.PkgReasonExplicit
		if info.Reason == dep.Dep {
			alpmReason = alpm.PkgReasonDepend
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
			})
		} else if info.Source == dep.Sync {
			repoUp.Up = append(repoUp.Up, Upgrade{
				Name:          name,
				RemoteVersion: info.Version,
				Repository:    *info.SyncDBName,
				Base:          "",
				LocalVersion:  info.LocalVersion,
				Reason:        alpmReason,
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
		graph = topo.New[string, *dep.InstallInfo]()
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
	allUpLen := graph.Len()
	aurUp, repoUp := u.graphToUpSlice(graph)

	sort.Sort(repoUp)
	sort.Sort(aurUp)

	allUp := UpSlice{Up: append(repoUp.Up, aurUp.Up...), Repos: append(repoUp.Repos, aurUp.Repos...)}

	u.log.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold(gotext.Get("Packages to upgrade.")))
	allUp.Print(u.log)

	u.log.Infoln(gotext.Get("Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)"))
	u.log.Warnln(gotext.Get("May cause partial upgrades and break systems"))

	numbers, err := u.log.GetInput(u.cfg.AnswerUpgrade, settings.NoConfirm)
	if err != nil {
		return nil, err
	}

	// upgrade menu asks you which packages to NOT upgrade so in this case
	// exclude and include are kind of swapped
	exclude, include, otherExclude, otherInclude := intrange.ParseNumberMenu(numbers)
	isInclude := len(include) == 0 && len(otherInclude) == 0

	excluded := make([]string, 0)
	for i := range allUp.Up {
		up := &allUp.Up[i]
		if isInclude && otherExclude.Get(up.Repository) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
		}

		if isInclude && exclude.Get(allUpLen-i) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
			continue
		}

		if !isInclude && !(include.Get(allUpLen-i) || otherInclude.Get(up.Repository)) {
			u.log.Debugln("pruning", up.Name)
			excluded = append(excluded, graph.Prune(up.Name)...)
			continue
		}
	}

	return excluded, nil
}
