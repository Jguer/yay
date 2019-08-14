package upgrade

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"

	rpc "github.com/mikkeloscar/aur"
)

const smallArrow = " ->"
const arrow = "==>"

func printIgnoringPackage(pkg alpm.Package, newPkgVersion string) {
	left, right := getVersionDiff(pkg.Version(), newPkgVersion)

	fmt.Printf("%s %s: ignoring package upgrade (%s => %s)\n",
		text.Yellow(text.Bold(smallArrow)),
		text.Cyan(pkg.Name()),
		left, right,
	)
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func upRepo(local []alpm.Package, alpmHandle *alpm.Handle, cmdArgs *types.Arguments) (UpSlice, error) {
	slice := UpSlice{}

	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return slice, err
	}

	err = alpmHandle.TransInit(alpm.TransFlagNoLock)
	if err != nil {
		return slice, err
	}

	defer alpmHandle.TransRelease()

	alpmHandle.SyncSysupgrade(cmdArgs.ExistsDouble("u", "sysupgrade"))
	alpmHandle.TransGetAdd().ForEach(func(pkg alpm.Package) error {
		localVer := "-"

		if localPkg := localDB.Pkg(pkg.Name()); localPkg != nil {
			localVer = localPkg.Version()
		}

		slice = append(slice, Upgrade{
			Name:          pkg.Name(),
			Repository:    pkg.DB().Name(),
			LocalVersion:  localVer,
			RemoteVersion: pkg.Version(),
		})
		return nil
	})

	return slice, nil
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, aurdata map[string]*rpc.Pkg, config *runtime.Configuration) (UpSlice, error) {
	toUpgrade := make(UpSlice, 0)

	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		if (config.TimeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
			(alpm.VerCmp(pkg.Version(), aurPkg.Version) < 0) {
			if pkg.ShouldIgnore() {
				printIgnoringPackage(pkg, aurPkg.Version)
			} else {
				toUpgrade = append(toUpgrade, Upgrade{
					Name:          aurPkg.Name,
					Repository:    "aur",
					LocalVersion:  pkg.Version(),
					RemoteVersion: aurPkg.Version})
			}
		}
	}

	return toUpgrade, nil
}

// upDevel returns a list of packages to upgrade from devel group
func upDevel(remote []alpm.Package, aurdata map[string]*rpc.Pkg, savedInfo vcs.InfoStore, config *runtime.Configuration) (toUpgrade UpSlice) {
	toUpdate := make([]alpm.Package, 0)
	toRemove := make([]string, 0)

	var mux1 sync.Mutex
	var mux2 sync.Mutex
	var wg sync.WaitGroup

	checkUpdate := func(vcsName string, e vcs.SHAInfos) {
		defer wg.Done()

		if e.NeedsUpdate(config) {
			if _, ok := aurdata[vcsName]; ok {
				for _, pkg := range remote {
					if pkg.Name() == vcsName {
						mux1.Lock()
						toUpdate = append(toUpdate, pkg)
						mux1.Unlock()
						return
					}
				}
			}

			mux2.Lock()
			toRemove = append(toRemove, vcsName)
			mux2.Unlock()
		}
	}

	for vcsName, e := range savedInfo {
		wg.Add(1)
		go checkUpdate(vcsName, e)
	}

	wg.Wait()

	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade = append(toUpgrade, Upgrade{
				Name:          pkg.Name(),
				Repository:    "devel",
				LocalVersion:  pkg.Version(),
				RemoteVersion: "latest-commit"})
		}
	}

	savedInfo.RemovePackage(toRemove)
	return
}

func isDevelName(name string) bool {
	for _, suffix := range []string{"git", "svn", "hg", "bzr", "nightly"} {
		if strings.HasSuffix(name, "-"+suffix) {
			return true
		}
	}

	return strings.Contains(name, "-always-")
}
func printLocalNewerThanAUR(
	remote []alpm.Package, aurdata map[string]*rpc.Pkg) {
	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		left, right := getVersionDiff(pkg.Version(), aurPkg.Version)

		if !isDevelName(pkg.Name()) && alpm.VerCmp(pkg.Version(), aurPkg.Version) > 0 {
			fmt.Printf("%s %s: local (%s) is newer than AUR (%s)\n",
				text.Yellow(text.Bold(smallArrow)),
				text.Cyan(pkg.Name()),
				left, right,
			)
		}
	}
}

// UpList returns lists of packages to upgrade from each source.
func UpList(config *runtime.Configuration, alpmHandle *alpm.Handle, cmdArgs *types.Arguments, savedInfo vcs.InfoStore, warnings *types.AURWarnings) (UpSlice, UpSlice, error) {
	local, remote, _, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var develUp UpSlice
	var repoUp UpSlice
	var aurUp UpSlice

	var errs types.MultiError

	aurdata := make(map[string]*rpc.Pkg)

	if config.Mode.IsAnyOrRepo() {
		fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Searching databases for updates...")))
		wg.Add(1)
		go func() {
			repoUp, err = upRepo(local, alpmHandle, cmdArgs)
			errs.Add(err)
			wg.Done()
		}()
	}

	if config.Mode.IsAnyOrAUR() {
		fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Searching AUR for updates...")))

		var _aurdata []*rpc.Pkg
		_aurdata, err = query.AURInfo(config, remoteNames, warnings)
		errs.Add(err)
		if err == nil {
			for _, pkg := range _aurdata {
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)
			go func() {
				aurUp, err = upAUR(remote, aurdata, config)
				errs.Add(err)
				wg.Done()
			}()

			if config.Devel {
				fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Checking development packages...")))
				wg.Add(1)
				go func() {
					develUp = upDevel(remote, aurdata, savedInfo, config)
					wg.Done()
				}()
			}
		}
	}

	wg.Wait()

	printLocalNewerThanAUR(remote, aurdata)

	if develUp != nil {
		names := make(types.StringSet)
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
