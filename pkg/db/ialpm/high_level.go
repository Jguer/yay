package ialpm

import (
	alpm "github.com/Jguer/go-alpm/v2"
)

// GetPackageNamesBySource returns package names with and without correspondence in SyncDBS respectively.
func (ae *AlpmExecutor) getPackageNamesBySource() {
	for _, localpkg := range ae.LocalPackages() {
		pkgName := localpkg.Name()
		if ae.SyncPackage(pkgName) != nil {
			ae.installedSyncPkgNames = append(ae.installedSyncPkgNames, pkgName)
		} else {
			ae.installedRemotePkgNames = append(ae.installedRemotePkgNames, pkgName)
			ae.installedRemotePkgMap[pkgName] = localpkg
		}
	}

	ae.log.Debugln("populating db executor package caches.",
		"sync_len", len(ae.installedSyncPkgNames), "remote_len", len(ae.installedRemotePkgNames))
}

func (ae *AlpmExecutor) InstalledRemotePackages() map[string]alpm.IPackage {
	if ae.installedRemotePkgMap == nil {
		ae.getPackageNamesBySource()
	}

	return ae.installedRemotePkgMap
}

func (ae *AlpmExecutor) InstalledRemotePackageNames() []string {
	if ae.installedRemotePkgNames == nil {
		ae.getPackageNamesBySource()
	}

	return ae.installedRemotePkgNames
}

func (ae *AlpmExecutor) InstalledSyncPackageNames() []string {
	if ae.installedSyncPkgNames == nil {
		ae.getPackageNamesBySource()
	}

	return ae.installedSyncPkgNames
}
