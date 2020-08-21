package main

import (
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/dep"
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

	info, err := query.AURInfoPrint(remoteNames, config.RequestSplitN)
	if err != nil {
		return err
	}

	bases := dep.GetBases(info)
	toSkip := pkgbuildsToSkip(bases, stringset.FromSlice(remoteNames))
	_, err = downloadPkgbuilds(bases, toSkip, config.BuildDir)
	if err != nil {
		return err
	}

	srcinfos, err := parseSrcinfoFiles(bases, false)
	if err != nil {
		return err
	}

	for i := range srcinfos {
		for iP := range srcinfos[i].Packages {
			wg.Add(1)
			go savedInfo.Update(srcinfos[i].Packages[iP].Pkgname, srcinfos[i].Source, &mux, &wg)
		}
	}

	wg.Wait()
	text.OperationInfoln(gotext.Get("GenDB finished. No packages were installed"))
	return err
}
