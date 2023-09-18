package dep

import (
	"context"

	"github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/text"
)

var (
	_ SourceHandler = &AllSyncHandler{}
	_ SourceHandler = &AllSyncGroupHandler{}
)

type AllSyncHandler struct {
	log          *text.Logger
	db           db.Executor
	foundTargets []alpm.IPackage
}

func (h *AllSyncHandler) Test(target Target) bool {
	if pkg := h.db.SyncSatisfier(target.Name); pkg != nil {
		h.foundTargets = append(h.foundTargets, pkg)
		return true
	}

	return false
}

func (h *AllSyncHandler) Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error {
	for _, pkg := range h.foundTargets {
		graphSyncPkg(ctx, h.db, graph, h.log, pkg, nil)
	}
	return nil
}

type SyncHandler struct {
	log         *text.Logger
	db          db.Executor
	foundPkgs   []alpm.IPackage
	foundGroups []Target
}

func (h *SyncHandler) Test(target Target) bool {
	pkg, err := h.db.SatisfierFromDB(target.Name, target.DB)
	if err != nil {
		h.log.Warnln("Unable to search DB", err)
		return false
	}

	if pkg != nil {
		h.foundPkgs = append(h.foundPkgs, pkg)
		return true
	}

	groupPackages, err := h.db.PackagesFromGroupAndDB(target.Name, target.DB)
	if err != nil {
		h.log.Warnln("Unable to search DB", err)
		return false
	}

	if len(groupPackages) > 0 {
		h.foundGroups = append(h.foundGroups, Target{DB: target.DB, Name: target.Name})
		return true
	}

	return false
}

func (h *SyncHandler) Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error {
	for _, pkg := range h.foundPkgs {
		graphSyncPkg(ctx, h.db, graph, h.log, pkg, nil)
	}

	for _, target := range h.foundGroups {
		GraphSyncGroup(ctx, graph, target.Name, target.DB)
	}
	return nil
}

type AllSyncGroupHandler struct {
	db           db.Executor
	foundTargets []Target
}

func (h *AllSyncGroupHandler) Test(target Target) bool {
	groupPackages := h.db.PackagesFromGroup(target.Name)
	if len(groupPackages) > 0 {
		dbName := groupPackages[0].DB().Name()
		h.foundTargets = append(h.foundTargets, Target{DB: dbName, Name: target.Name})
		return true
	}

	return false
}

func (h *AllSyncGroupHandler) Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error {
	for _, target := range h.foundTargets {
		GraphSyncGroup(ctx, graph, target.Name, target.DB)
	}
	return nil
}

func graphSyncPkg(ctx context.Context, dbExecutor db.Executor,
	graph *topo.Graph[string, *InstallInfo], logger *text.Logger,
	pkg alpm.IPackage, upgradeInfo *db.SyncUpgrade,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(pkg.Name())
	_ = pkg.Provides().ForEach(func(p *alpm.Depend) error {
		logger.Debugln(pkg.Name() + " provides: " + p.String())
		graph.Provides(p.Name, p, pkg.Name())
		return nil
	})

	dbName := pkg.DB().Name()
	info := &InstallInfo{
		Source:     Sync,
		Reason:     Explicit,
		Version:    pkg.Version(),
		SyncDBName: &dbName,
	}

	if upgradeInfo == nil {
		if localPkg := dbExecutor.LocalPackage(pkg.Name()); localPkg != nil {
			info.Reason = Reason(localPkg.Reason())
		}
	} else {
		info.Upgrade = true
		info.Reason = Reason(upgradeInfo.Reason)
		info.LocalVersion = upgradeInfo.LocalVersion
	}

	validateAndSetNodeInfo(graph, pkg.Name(), &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[info.Reason],
		Background: bgColorMap[info.Source],
		Value:      info,
	})

	return graph
}

func GraphSyncGroup(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	groupName, dbName string,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(groupName)

	validateAndSetNodeInfo(graph, groupName, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[Explicit],
		Background: bgColorMap[Sync],
		Value: &InstallInfo{
			Source:     Sync,
			Reason:     Explicit,
			Version:    "",
			SyncDBName: &dbName,
			IsGroup:    true,
		},
	})

	return graph
}
