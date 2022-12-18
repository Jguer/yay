package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

// createDevelDB forces yay to create a DB of the existing development packages.
func createDevelDB(ctx context.Context, config *settings.Configuration, dbExecutor db.Executor) error {
	remoteNames := dbExecutor.InstalledRemotePackageNames()
	info, err := query.AURInfoPrint(ctx, config.Runtime.AURClient, remoteNames, config.RequestSplitN)
	if err != nil {
		return err
	}

	bases := dep.GetBases(info)
	toSkip := pkgbuildsToSkip(bases, stringset.FromSlice(remoteNames))

	targets := make([]string, 0, len(bases))
	pkgBuildDirsByBase := make(map[string]string, len(bases))

	for _, base := range bases {
		if !toSkip.Get(base.Pkgbase()) {
			targets = append(targets, base.Pkgbase())
		}

		pkgBuildDirsByBase[base.Pkgbase()] = filepath.Join(config.BuildDir, base.Pkgbase())
	}

	toSkipSlice := toSkip.ToSlice()
	if len(toSkipSlice) != 0 {
		text.OperationInfoln(
			gotext.Get("PKGBUILD up to date, Skipping (%d/%d): %s",
				len(toSkipSlice), len(bases), text.Cyan(strings.Join(toSkipSlice, ", "))))
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		config.Runtime.CmdBuilder, targets, config.AURURL, config.BuildDir, false); errA != nil {
		return err
	}

	srcinfos, err := parseSrcinfoFiles(pkgBuildDirsByBase, false)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := range srcinfos {
		for iP := range srcinfos[i].Packages {
			wg.Add(1)

			go func(i string, iP int) {
				config.Runtime.VCSStore.Update(ctx, srcinfos[i].Packages[iP].Pkgname, srcinfos[i].Source)
				wg.Done()
			}(i, iP)
		}
	}

	wg.Wait()
	text.OperationInfoln(gotext.Get("GenDB finished. No packages were installed"))

	return err
}
