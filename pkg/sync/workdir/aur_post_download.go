package workdir

import (
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/runtime"
	settingslua "github.com/Jguer/yay/v12/pkg/settings/lua"

	mapset "github.com/deckarep/golang-set/v2"
)

func runAURPostDownloadLuaHooks(run *runtime.Runtime, pkgbuildDirsByBase map[string]string,
	installed mapset.Set[string], targets []map[string]*dep.InstallInfo,
) error {
	if run == nil || run.Lua == nil || !run.Lua.HasAutocmd(settingslua.EventAURPostDownload) {
		return nil
	}

	events, err := aurPostDownloadEvents(pkgbuildDirsByBase, installed, targets)
	if err != nil {
		return err
	}

	for i := range events {
		if err := run.Lua.RunAURPostDownload(&events[i]); err != nil {
			return err
		}
	}

	return nil
}

func aurPostDownloadEvents(pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
	targets []map[string]*dep.InstallInfo,
) ([]settingslua.AURPreInstallEvent, error) {
	packagesByBase := aurTargetPackagesByBase(targets)
	bases := sortedAURBases(pkgbuildDirsByBase)

	events := make([]settingslua.AURPreInstallEvent, 0, len(bases))
	for _, base := range bases {
		event, err := aurPackageEvent(settingslua.EventAURPostDownload, base, pkgbuildDirsByBase[base],
			packagesByBase[base], installed, targets)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}
