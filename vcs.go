package main

import (
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/download"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

// createDevelDB forces yay to create a DB of the existing development packages
func createDevelDB(config *settings.Configuration, dbExecutor db.Executor) error {
	var mux sync.Mutex
	var wg sync.WaitGroup

	_, remoteNames, err := query.GetPackageNamesBySource(dbExecutor)
	if err != nil {
		return err
	}

	info, err := query.AURInfoPrint(config.Runtime.AURClient, remoteNames, config.RequestSplitN)
	if err != nil {
		return err
	}

	bases := dep.GetBases(info)
	toSkip := pkgbuildsToSkip(bases, stringset.FromSlice(remoteNames))

	targets := make([]string, 0, len(bases))
	for _, base := range bases {
		if !toSkip.Get(base.Pkgbase()) {
			targets = append(targets, base.Pkgbase())
		}
	}

	toSkipSlice := toSkip.ToSlice()
	if len(toSkipSlice) != 0 {
		text.OperationInfoln(
			gotext.Get("PKGBUILD up to date, Skipping (%d/%d): %s",
				len(toSkipSlice), len(bases), text.Cyan(strings.Join(toSkipSlice, ", "))))
	}

	if _, errA := download.AURPKGBUILDRepos(config.Runtime.CmdRunner,
		config.Runtime.CmdBuilder, targets, config.AURURL, config.BuildDir, false); errA != nil {
		return err
	}

	srcinfos, err := parseSrcinfoFiles(bases, false)
	if err != nil {
		return err
	}

	for i := range srcinfos {
		for iP := range srcinfos[i].Packages {
			wg.Add(1)
			go config.Runtime.VCSStore.Update(srcinfos[i].Packages[iP].Pkgname, srcinfos[i].Source, &mux, &wg)
		}
	}

	wg.Wait()
	text.OperationInfoln(gotext.Get("GenDB finished. No packages were installed"))
	return err
}
