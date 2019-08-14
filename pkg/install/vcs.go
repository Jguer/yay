package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/download"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"
	gosrc "github.com/Morganamilo/go-srcinfo"
)

// CreateDevelDB forces yay to create a DB of the existing development packages
func CreateDevelDB(alpmHandle *alpm.Handle, config *runtime.Configuration, savedInfo vcs.InfoStore) error {
	var mux sync.Mutex
	var wg sync.WaitGroup

	_, _, _, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	info, err := query.AURInfoPrint(config, remoteNames)
	if err != nil {
		return err
	}

	bases := types.GetBases(info)
	toSkip := pkgbuildsToSkip(config, bases, types.SliceToStringSet(remoteNames))
	download.Pkgbuilds(config, bases, toSkip, config.BuildDir)
	srcinfos, _ := parseSrcinfoFiles(bases, config.BuildDir, false)

	for _, pkgbuild := range srcinfos {
		for _, pkg := range pkgbuild.Packages {
			wg.Add(1)
			go savedInfo.Update(config, pkg.Pkgname, pkgbuild.Source, &mux, &wg)
		}
	}

	wg.Wait()
	fmt.Println(text.Bold(text.Yellow(arrow) + text.Bold(" GenDB finished. No packages were installed")))
	return err
}

func parseSrcinfoFiles(bases []types.Base, dir string, errIsFatal bool) (map[string]*gosrc.Srcinfo, error) {
	srcinfos := make(map[string]*gosrc.Srcinfo)
	for k, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(dir, pkg)

		str := text.Bold(text.Cyan("::") + " Parsing SRCINFO (%d/%d): %s\n")
		fmt.Printf(str, k+1, len(bases), text.Cyan(base.String()))

		pkgbuild, err := gosrc.ParseFile(filepath.Join(dir, ".SRCINFO"))
		if err != nil {
			if !errIsFatal {
				fmt.Fprintf(os.Stderr, "failed to parse %s -- skipping: %s\n", base.String(), err)
				continue
			}
			return nil, fmt.Errorf("failed to parse %s: %s", base.String(), err)
		}

		srcinfos[pkg] = pkgbuild
	}

	return srcinfos, nil
}

func pkgbuildsToSkip(config *runtime.Configuration, bases []types.Base, targets types.StringSet) types.StringSet {
	toSkip := make(types.StringSet)

	for _, base := range bases {
		isTarget := false
		for _, pkg := range base {
			isTarget = isTarget || targets.Get(pkg.Name)
		}

		if (config.ReDownload == "yes" && isTarget) || config.ReDownload == "all" {
			continue
		}

		dir := filepath.Join(config.BuildDir, base.Pkgbase(), ".SRCINFO")
		pkgbuild, err := gosrc.ParseFile(dir)

		if err == nil {
			if alpm.VerCmp(pkgbuild.Version(), base.Version()) >= 0 {
				toSkip.Set(base.Pkgbase())
			}
		}
	}

	return toSkip
}
