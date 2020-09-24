package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/upgrade"
	"github.com/Jguer/yay/v10/pkg/vcs"

	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/stringset"
)

// upList returns lists of packages to upgrade from each source.
func upList(warnings *query.AURWarnings, dbExecutor db.Executor, enableDowngrade bool) (aurUp, repoUp upgrade.UpSlice, err error) {
	remote, remoteNames := query.GetRemotePackages(dbExecutor)

	var wg sync.WaitGroup
	var develUp upgrade.UpSlice
	var errs multierror.MultiError

	aurdata := make(map[string]*rpc.Pkg)

	for _, pkg := range remote {
		if pkg.ShouldIgnore() {
			warnings.Ignore.Set(pkg.Name())
		}
	}

	if config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeRepo {
		text.OperationInfoln(gotext.Get("Searching databases for updates..."))
		wg.Add(1)
		go func() {
			repoUp, err = dbExecutor.RepoUpgrades(enableDowngrade)
			errs.Add(err)
			wg.Done()
		}()
	}

	if config.Runtime.Mode == settings.ModeAny || config.Runtime.Mode == settings.ModeAUR {
		text.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		var _aurdata []*rpc.Pkg
		_aurdata, err = query.AURInfo(remoteNames, warnings, config.RequestSplitN)
		errs.Add(err)
		if err == nil {
			for _, pkg := range _aurdata {
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)
			go func() {
				aurUp = upAUR(remote, aurdata, config.TimeUpdate)
				wg.Done()
			}()

			if config.Devel {
				text.OperationInfoln(gotext.Get("Checking development packages..."))
				wg.Add(1)
				go func() {
					develUp = upDevel(remote, aurdata, config.Runtime.VCSStore)
					wg.Done()
				}()
			}
		}
	}

	wg.Wait()

	printLocalNewerThanAUR(remote, aurdata)

	if develUp != nil {
		names := make(stringset.StringSet)
		for _, up := range develUp {
			names.Set(up.Name)
		}
		for _, up := range aurUp {
			if !names.Get(up.Name) {
				develUp = append(develUp, up)
			}
		}

		aurUp = develUp
	}

	return aurUp, repoUp, errs.Return()
}

func upDevel(
	remote []db.RepoPackage,
	aurdata map[string]*rpc.Pkg,
	localCache *vcs.InfoStore) upgrade.UpSlice {
	toUpdate := make([]db.RepoPackage, 0, len(aurdata))
	toRemove := make([]string, 0)

	var mux1, mux2 sync.Mutex
	var wg sync.WaitGroup

	checkUpdate := func(pkgName string, e vcs.OriginInfoByURL) {
		defer wg.Done()

		if localCache.NeedsUpdate(e) {
			if _, ok := aurdata[pkgName]; ok {
				for _, pkg := range remote {
					if pkg.Name() == pkgName {
						mux1.Lock()
						toUpdate = append(toUpdate, pkg)
						mux1.Unlock()
						return
					}
				}
			}

			mux2.Lock()
			toRemove = append(toRemove, pkgName)
			mux2.Unlock()
		}
	}

	for pkgName, e := range localCache.OriginsByPackage {
		wg.Add(1)
		go checkUpdate(pkgName, e)
	}

	wg.Wait()

	toUpgrade := make(upgrade.UpSlice, 0, len(toUpdate))
	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade = append(toUpgrade,
				upgrade.Upgrade{
					Name:          pkg.Name(),
					Repository:    "devel",
					LocalVersion:  pkg.Version(),
					RemoteVersion: "latest-commit",
				})
		}
	}

	localCache.RemovePackage(toRemove)
	return toUpgrade
}

func printIgnoringPackage(pkg db.RepoPackage, newPkgVersion string) {
	left, right := upgrade.GetVersionDiff(pkg.Version(), newPkgVersion)

	text.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		text.Cyan(pkg.Name()),
		left, right,
	))
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []db.RepoPackage, aurdata map[string]*rpc.Pkg, timeUpdate bool) upgrade.UpSlice {
	toUpgrade := make(upgrade.UpSlice, 0)

	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		if (timeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
			(alpm.VerCmp(pkg.Version(), aurPkg.Version) < 0) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(pkg, aurPkg.Version)
			} else {
				toUpgrade = append(toUpgrade,
					upgrade.Upgrade{
						Name:          aurPkg.Name,
						Repository:    "aur",
						LocalVersion:  pkg.Version(),
						RemoteVersion: aurPkg.Version,
					})
			}
		}
	}

	return toUpgrade
}

func printLocalNewerThanAUR(
	remote []db.RepoPackage, aurdata map[string]*rpc.Pkg) {
	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		left, right := upgrade.GetVersionDiff(pkg.Version(), aurPkg.Version)

		if !isDevelPackage(pkg) && alpm.VerCmp(pkg.Version(), aurPkg.Version) > 0 {
			text.Warnln(gotext.Get("%s: local (%s) is newer than AUR (%s)",
				text.Cyan(pkg.Name()),
				left, right,
			))
		}
	}
}

func isDevelName(name string) bool {
	for _, suffix := range []string{"git", "svn", "hg", "bzr", "nightly"} {
		if strings.HasSuffix(name, "-"+suffix) {
			return true
		}
	}

	return strings.Contains(name, "-always-")
}

func isDevelPackage(pkg db.RepoPackage) bool {
	return isDevelName(pkg.Name()) || isDevelName(pkg.Base())
}

// upgradePkgs handles updating the cache and installing updates.
func upgradePkgs(aurUp, repoUp upgrade.UpSlice) (ignore, aurNames stringset.StringSet, err error) {
	ignore = make(stringset.StringSet)
	aurNames = make(stringset.StringSet)

	allUpLen := len(repoUp) + len(aurUp)
	if allUpLen == 0 {
		return ignore, aurNames, nil
	}

	if !config.UpgradeMenu {
		for _, pkg := range aurUp {
			aurNames.Set(pkg.Name)
		}

		return ignore, aurNames, nil
	}

	sort.Sort(repoUp)
	sort.Sort(aurUp)
	allUp := append(repoUp, aurUp...)
	fmt.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold(gotext.Get("Packages to upgrade.")))
	allUp.Print()

	text.Infoln(gotext.Get("Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)"))

	numbers, err := getInput(config.AnswerUpgrade)
	if err != nil {
		return nil, nil, err
	}

	// upgrade menu asks you which packages to NOT upgrade so in this case
	// include and exclude are kind of swapped
	include, exclude, otherInclude, otherExclude := intrange.ParseNumberMenu(numbers)

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoUp {
		if isInclude && otherInclude.Get(pkg.Repository) {
			ignore.Set(pkg.Name)
		}

		if isInclude && !include.Get(len(repoUp)-i+len(aurUp)) {
			continue
		}

		if !isInclude && (exclude.Get(len(repoUp)-i+len(aurUp)) || otherExclude.Get(pkg.Repository)) {
			continue
		}

		ignore.Set(pkg.Name)
	}

	for i, pkg := range aurUp {
		if isInclude && otherInclude.Get(pkg.Repository) {
			continue
		}

		if isInclude && !include.Get(len(aurUp)-i) {
			aurNames.Set(pkg.Name)
		}

		if !isInclude && (exclude.Get(len(aurUp)-i) || otherExclude.Get(pkg.Repository)) {
			aurNames.Set(pkg.Name)
		}
	}

	return ignore, aurNames, err
}
