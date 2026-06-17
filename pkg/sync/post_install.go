package sync

import (
	"maps"
	"slices"

	"github.com/Jguer/yay/v13/pkg/dep"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"
)

// postInstallEvent flattens the resolved topo layers into the PostInstall
// payload. Packages recorded in failedAndIgnored (last-layer AUR build
// failures tolerated by the installer) are marked installed = false.
func postInstallEvent(targets []map[string]*dep.InstallInfo, failedAndIgnored map[string]error) *settingslua.PostInstallEvent {
	merged := map[string]*dep.InstallInfo{}
	for _, layer := range targets {
		maps.Copy(merged, layer)
	}

	names := slices.Sorted(maps.Keys(merged))

	packages := make([]settingslua.PostInstallPackage, 0, len(names))
	for _, name := range names {
		info := merged[name]
		_, failed := failedAndIgnored[name]
		packages = append(packages, settingslua.PostInstallPackage{
			Name:         name,
			Version:      info.Version,
			LocalVersion: info.LocalVersion,
			Source:       luaSource(info.Source),
			Reason:       luaReason(info.Reason),
			Installed:    !failed,
			Upgrade:      info.Upgrade,
			Devel:        info.Devel,
		})
	}

	return &settingslua.PostInstallEvent{Packages: packages}
}

func luaSource(source dep.Source) string {
	switch source {
	case dep.AUR:
		return "aur"
	case dep.Sync:
		return "sync"
	case dep.Local:
		return "local"
	case dep.SrcInfo:
		return "srcinfo"
	default:
		return "missing"
	}
}

func luaReason(reason dep.Reason) string {
	switch reason {
	case dep.Explicit:
		return "explicit"
	case dep.Dep:
		return "dependency"
	case dep.MakeDep:
		return "make_dependency"
	case dep.CheckDep:
		return "check_dependency"
	default:
		return "unknown"
	}
}
