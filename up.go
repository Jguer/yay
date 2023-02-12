package main

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/go-alpm/v2"
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"
	"github.com/Jguer/yay/v11/pkg/upgrade"
	"github.com/Jguer/yay/v11/pkg/vcs"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type UpgradeService struct {
	w          io.Writer
	grapher    *dep.Grapher
	aurCache   settings.AURCache
	aurClient  aur.ClientInterface
	dbExecutor db.Executor
	vcsStore   vcs.Store
	runtime    *settings.Runtime
	cfg        *settings.Configuration
}

func NewUpgradeService(w io.Writer, grapher *dep.Grapher, aurCache settings.AURCache,
	aurClient aur.ClientInterface, dbExecutor db.Executor,
	vcsStore vcs.Store, runtime *settings.Runtime, cfg *settings.Configuration,
) *UpgradeService {
	return &UpgradeService{
		w:          w,
		grapher:    grapher,
		aurCache:   aurCache,
		aurClient:  aurClient,
		dbExecutor: dbExecutor,
		vcsStore:   vcsStore,
		runtime:    runtime,
		cfg:        cfg,
	}
}

// upGraph adds packages to upgrade to the graph.
func (u *UpgradeService) upGraph(ctx context.Context, graph *topo.Graph[string, *dep.InstallInfo],
	warnings *query.AURWarnings, enableDowngrade bool,
	filter upgrade.Filter,
) (err error) {
	var (
		develUp upgrade.UpSlice
		errs    multierror.MultiError
		aurdata = make(map[string]*aur.Pkg)
		aurUp   upgrade.UpSlice
	)

	remote := u.dbExecutor.InstalledRemotePackages()
	remoteNames := u.dbExecutor.InstalledRemotePackageNames()

	if u.runtime.Mode.AtLeastAUR() {
		if u.cfg.Devel {
			text.OperationInfoln(gotext.Get("Checking development packages..."))

			develUp = upgrade.UpDevel(ctx, remote, aurdata, u.vcsStore)

			u.vcsStore.CleanOrphans(remote)
		}

		text.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		var _aurdata []aur.Pkg
		if u.aurCache != nil {
			_aurdata, err = u.aurCache.Get(ctx, &metadata.AURQuery{Needles: remoteNames, By: aur.Name})
		} else {
			_aurdata, err = query.AURInfo(ctx, u.aurClient, remoteNames, warnings, u.cfg.RequestSplitN)
		}

		errs.Add(err)

		if err == nil {
			for i := range _aurdata {
				pkg := &_aurdata[i]
				aurdata[pkg.Name] = pkg
			}

			aurUp = upgrade.UpAUR(remote, aurdata, u.cfg.TimeUpdate)
		}
	}

	names := mapset.NewThreadUnsafeSet[string]()
	for _, up := range develUp.Up {
		// check if deps are satisfied for aur packages
		reason := dep.Explicit
		if up.Reason == alpm.PkgReasonDepend {
			reason = dep.Dep
		}

		if filter != nil && !filter(&up) {
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
		})
		names.Add(up.Name)
	}

	for _, up := range aurUp.Up {
		// add devel packages if they are not already in the list
		if names.Contains(up.Name) {
			continue
		}

		// check if deps are satisfied for aur packages
		reason := dep.Explicit
		if up.Reason == alpm.PkgReasonDepend {
			reason = dep.Dep
		}

		if filter != nil && !filter(&up) {
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
		text.OperationInfoln(gotext.Get("Searching databases for updates..."))

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

func (u *UpgradeService) graphToUpSlice(graph *topo.Graph[string, *dep.InstallInfo]) (aurUp, repoUp upgrade.UpSlice) {
	aurUp = upgrade.UpSlice{Up: make([]upgrade.Upgrade, 0, graph.Len())}
	repoUp = upgrade.UpSlice{Up: make([]upgrade.Upgrade, 0, graph.Len()), Repos: u.dbExecutor.Repos()}

	graph.ForEach(func(name string, info *dep.InstallInfo) error {
		alpmReason := alpm.PkgReasonExplicit
		if info.Reason == dep.Dep {
			alpmReason = alpm.PkgReasonDepend
		}

		if info.Source == dep.AUR {
			aurRepo := "aur"
			if info.Devel {
				aurRepo = "devel"
			}
			aurUp.Up = append(aurUp.Up, upgrade.Upgrade{
				Name:          name,
				RemoteVersion: info.Version,
				Repository:    aurRepo,
				Base:          *info.AURBase,
				LocalVersion:  info.LocalVersion,
				Reason:        alpmReason,
			})
		} else if info.Source == dep.Sync {
			repoUp.Up = append(repoUp.Up, upgrade.Upgrade{
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

func (u *UpgradeService) sysupgradeTargetsV2(ctx context.Context,
	graph *topo.Graph[string, *dep.InstallInfo],
	enableDowngrade bool,
) (*topo.Graph[string, *dep.InstallInfo], stringset.StringSet, error) {
	warnings := query.NewWarnings()

	err := u.upGraph(ctx, graph, warnings, enableDowngrade,
		func(*upgrade.Upgrade) bool { return true })
	if err != nil {
		return graph, nil, err
	}

	warnings.Print()

	ignore := make(stringset.StringSet)

	allUpLen := graph.Len()
	if allUpLen == 0 {
		return graph, ignore, nil
	}

	// TODO: refactor print to not use upgrade.UpSlice
	aurUp, repoUp := u.graphToUpSlice(graph)

	sort.Sort(repoUp)
	sort.Sort(aurUp)

	allUp := upgrade.UpSlice{Up: append(repoUp.Up, aurUp.Up...), Repos: append(repoUp.Repos, aurUp.Repos...)}

	fmt.Fprintf(u.w, "%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold(gotext.Get("Packages to upgrade.")))
	allUp.Print()

	text.Infoln(gotext.Get("Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)"))

	numbers, err := text.GetInput(u.cfg.AnswerUpgrade, settings.NoConfirm)
	if err != nil {
		return nil, nil, err
	}

	// upgrade menu asks you which packages to NOT upgrade so in this case
	// exclude and include are kind of swapped
	exclude, include, otherExclude, otherInclude := intrange.ParseNumberMenu(numbers)
	text.Debugln("Include:", include, "Exclude:", exclude, "OtherInclude:", otherInclude, "OtherExclude:", otherExclude)

	isInclude := len(include) == 0 && len(otherInclude) == 0

	for i := range allUp.Up {
		u := &allUp.Up[i]
		if isInclude && otherExclude.Get(u.Repository) {
			ignore.Set(u.Name)
		}

		if isInclude && !exclude.Get(i) {
			dep.AddUpgradeToGraph(u, graph)
			continue
		}

		if !isInclude && (include.Get(i) || otherInclude.Get(u.Repository)) {
			dep.AddUpgradeToGraph(u, graph)
			continue
		}

		ignore.Set(u.Name)
	}

	return graph, ignore, err
}
