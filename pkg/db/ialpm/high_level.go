package ialpm

import (
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/text"
)

// GetPackageNamesBySource returns package names with and without correspondence in SyncDBS respectively.
func (ae *AlpmExecutor) getPackageNamesBySource() {
	for _, localpkg := range ae.LocalPackages() {
		pkgName := localpkg.Name()
		if ae.SyncPackage(pkgName) != nil {
			ae.installedSyncPkgNames = append(ae.installedSyncPkgNames, pkgName)
		} else {
			ae.installedRemotePkgs = append(ae.installedRemotePkgs, localpkg)
			ae.installedRemotePkgNames = append(ae.installedRemotePkgNames, pkgName)
		}
	}

	text.Debugln("populating db executor package caches.",
		"sync_len", len(ae.installedSyncPkgNames), "remote_len", len(ae.installedRemotePkgNames))
}

func (ae *AlpmExecutor) InstalledRemotePackages() []db.IPackage {
	if ae.installedRemotePkgs == nil {
		ae.getPackageNamesBySource()
	}

	return ae.installedRemotePkgs
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
