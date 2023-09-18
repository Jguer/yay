package dep

import (
	"context"

	aurc "github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/query"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

func (h *AURHandler) GraphUpgrades(ctx context.Context, graph *topo.Graph[string, *InstallInfo],
	enableDowngrade bool, filter Filter,
) error {
	h.log.OperationInfoln(gotext.Get("Searching AUR for updates..."))
	if h.cfg.Devel {
		h.log.OperationInfoln(gotext.Get("Checking development packages..."))
	}

	aurdata := make(map[string]*aur.Pkg)
	warnings := query.NewWarnings(h.log.Child("warnings"))
	remote := h.dbExecutor.InstalledRemotePackages()
	remoteNames := h.dbExecutor.InstalledRemotePackageNames()

	_aurdata, err := h.aurClient.Get(ctx, &aurc.Query{Needles: remoteNames, By: aurc.Name})
	if err != nil {
		return err
	}

	for i := range _aurdata {
		pkg := &_aurdata[i]
		aurdata[pkg.Name] = pkg
		warnings.AddToWarnings(remote, pkg)
	}

	h.upAUR(ctx, remote, aurdata, enableDowngrade, graph, filter)

	if h.cfg.Devel {
		h.vcsStore.CleanOrphans(remote)
	}

	warnings.CalculateMissing(remoteNames, remote, aurdata)

	return nil
}

// UpAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func (h *AURHandler) upAUR(ctx context.Context,
	remote map[string]db.IPackage, aurdata map[string]*query.Pkg,
	enableDowngrade bool, graph *topo.Graph[string, *InstallInfo],
	filter Filter,
) {
	aurPkgsAdded := make([]*aurc.Pkg, 0)
	for name, pkg := range remote {
		aurPkg, ok := aurdata[name]
		// Check for new versions
		if ok && (db.VerCmp(pkg.Version(), aurPkg.Version) < 0) ||
			(enableDowngrade && (db.VerCmp(pkg.Version(), aurPkg.Version) > 0)) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(h.log, pkg, aurPkg.Version)
			} else {
				// check if deps are satisfied for aur packages
				reason := Explicit
				if pkg.Reason() == alpm.PkgReasonDepend {
					reason = Dep
				}

				// FIXME: Reimplement filter
				graph = h.GraphAURTarget(ctx, graph, aurPkg, &InstallInfo{
					Reason:       reason,
					Source:       AUR,
					AURBase:      &aurPkg.PackageBase,
					Upgrade:      true,
					Version:      aurPkg.Version,
					LocalVersion: pkg.Version(),
				})
				aurPkgsAdded = append(aurPkgsAdded, aurPkg)
				continue
			}
		}

		if h.cfg.Devel {
			if h.vcsStore.ToUpgrade(ctx, name) {
				if _, ok := aurdata[name]; !ok {
					h.log.Warnln(gotext.Get("ignoring package devel upgrade (no AUR info found):"), name)
					continue
				}

				if pkg.ShouldIgnore() {
					printIgnoringPackage(h.log, pkg, "latest-commit")
					continue
				}

				// check if deps are satisfied for aur packages
				reason := Explicit
				if pkg.Reason() == alpm.PkgReasonDepend {
					reason = Dep
				}

				// FIXME: Reimplement filter
				graph = h.GraphAURTarget(ctx, graph, aurPkg, &InstallInfo{
					Reason:       reason,
					Source:       AUR,
					AURBase:      &aurPkg.PackageBase,
					Upgrade:      true,
					Version:      aurPkg.Version,
					LocalVersion: pkg.Version(),
				})
				aurPkgsAdded = append(aurPkgsAdded, aurPkg)
			}
		}

	}

	h.AddDepsForPkgs(ctx, aurPkgsAdded, graph)
}

func printIgnoringPackage(log *text.Logger, pkg db.IPackage, newPkgVersion string) {
	left, right := query.GetVersionDiff(pkg.Version(), newPkgVersion)

	pkgName := pkg.Name()
	log.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		text.Cyan(pkgName),
		left, right,
	))
}
