package dep

import (
	"context"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
)

func (h *AllSyncHandler) GraphUpgrades(ctx context.Context, graph *topo.Graph[string, *InstallInfo],
	enableDowngrade bool, filter Filter,
) error {
	h.log.OperationInfoln(gotext.Get("Searching databases for updates..."))

	syncUpgrades, err := h.db.SyncUpgrades(enableDowngrade)
	if err != nil {
		return err
	}

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
		graph = graphSyncPkg(ctx, h.db, graph, h.log, up.Package, &upgradeInfo)
	}

	return nil
}
