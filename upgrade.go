package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/intrange"
	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"

	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/stringset"
)

// upgrade type describes a system upgrade.
type upgrade struct {
	Name          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
}

// upSlice is a slice of Upgrades
type upSlice []upgrade

func (u upSlice) Len() int      { return len(u) }
func (u upSlice) Swap(i, j int) { u[i], u[j] = u[j], u[i] }

func (u upSlice) Less(i, j int) bool {
	if u[i].Repository == u[j].Repository {
		iRunes := []rune(u[i].Name)
		jRunes := []rune(u[j].Name)
		return text.LessRunes(iRunes, jRunes)
	}

	syncDB, err := config.Runtime.AlpmHandle.SyncDBs()
	if err != nil {
		iRunes := []rune(u[i].Repository)
		jRunes := []rune(u[j].Repository)
		return text.LessRunes(iRunes, jRunes)
	}

	less := false
	found := syncDB.ForEach(func(db alpm.DB) error {
		switch db.Name() {
		case u[i].Repository:
			less = true
		case u[j].Repository:
			less = false
		default:
			return nil
		}

		return fmt.Errorf("")
	})

	if found != nil {
		return less
	}

	iRunes := []rune(u[i].Repository)
	jRunes := []rune(u[j].Repository)
	return text.LessRunes(iRunes, jRunes)
}

func getVersionDiff(oldVersion, newVersion string) (left, right string) {
	if oldVersion == newVersion {
		return oldVersion + red(""), newVersion + green("")
	}

	diffPosition := 0

	checkWords := func(str string, index int, words ...string) bool {
		for _, word := range words {
			wordLength := len(word)
			nextIndex := index + 1
			if (index < len(str)-wordLength) &&
				(str[nextIndex:(nextIndex+wordLength)] == word) {
				return true
			}
		}
		return false
	}

	for index, char := range oldVersion {
		charIsSpecial := !(unicode.IsLetter(char) || unicode.IsNumber(char))

		if (index >= len(newVersion)) || (char != rune(newVersion[index])) {
			if charIsSpecial {
				diffPosition = index
			}
			break
		}

		if charIsSpecial ||
			(((index == len(oldVersion)-1) || (index == len(newVersion)-1)) &&
				((len(oldVersion) != len(newVersion)) ||
					(oldVersion[index] == newVersion[index]))) ||
			checkWords(oldVersion, index, "rc", "pre", "alpha", "beta") {
			diffPosition = index + 1
		}
	}

	samePart := oldVersion[0:diffPosition]

	left = samePart + red(oldVersion[diffPosition:])
	right = samePart + green(newVersion[diffPosition:])

	return left, right
}

// upList returns lists of packages to upgrade from each source.
func upList(warnings *query.AURWarnings, alpmHandle *alpm.Handle, enableDowngrade bool) (aurUp, repoUp upSlice, err error) {
	remote, remoteNames, err := query.GetRemotePackages(alpmHandle)
	if err != nil {
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var develUp upSlice
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
			repoUp, err = upRepo(alpmHandle, enableDowngrade)
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
				aurUp = upAUR(remote, aurdata)
				wg.Done()
			}()

			if config.Devel {
				text.OperationInfoln(gotext.Get("Checking development packages..."))
				wg.Add(1)
				go func() {
					develUp = upDevel(remote, aurdata)
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

func upDevel(remote []alpm.Package, aurdata map[string]*rpc.Pkg) upSlice {
	toUpdate := make([]alpm.Package, 0)
	toRemove := make([]string, 0)

	var mux1 sync.Mutex
	var mux2 sync.Mutex
	var wg sync.WaitGroup

	checkUpdate := func(vcsName string, e shaInfos) {
		defer wg.Done()

		if e.needsUpdate() {
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

	toUpgrade := make(upSlice, 0, len(toUpdate))
	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade = append(toUpgrade, upgrade{pkg.Name(), "devel", pkg.Version(), "latest-commit"})
		}
	}

	removeVCSPackage(toRemove)
	return toUpgrade
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, aurdata map[string]*rpc.Pkg) upSlice {
	toUpgrade := make(upSlice, 0)

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
				toUpgrade = append(toUpgrade, upgrade{aurPkg.Name, "aur", pkg.Version(), aurPkg.Version})
			}
		}
	}

	return toUpgrade
}

func printIgnoringPackage(pkg alpm.Package, newPkgVersion string) {
	left, right := getVersionDiff(pkg.Version(), newPkgVersion)

	text.Warnln(gotext.Get("%s: ignoring package upgrade (%s => %s)",
		cyan(pkg.Name()),
		left, right,
	))
}

func printLocalNewerThanAUR(
	remote []alpm.Package, aurdata map[string]*rpc.Pkg) {
	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		left, right := getVersionDiff(pkg.Version(), aurPkg.Version)

		if !isDevelPackage(pkg) && alpm.VerCmp(pkg.Version(), aurPkg.Version) > 0 {
			text.Warnln(gotext.Get("%s: local (%s) is newer than AUR (%s)",
				cyan(pkg.Name()),
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

func isDevelPackage(pkg alpm.Package) bool {
	return isDevelName(pkg.Name()) || isDevelName(pkg.Base())
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func upRepo(alpmHandle *alpm.Handle, enableDowngrade bool) (upSlice, error) {
	slice := upSlice{}

	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return slice, err
	}

	err = alpmHandle.TransInit(alpm.TransFlagNoLock)
	if err != nil {
		return slice, err
	}

	defer func() {
		err = alpmHandle.TransRelease()
	}()

	err = alpmHandle.SyncSysupgrade(enableDowngrade)
	if err != nil {
		return slice, err
	}
	_ = alpmHandle.TransGetAdd().ForEach(func(pkg alpm.Package) error {
		localVer := "-"

		if localPkg := localDB.Pkg(pkg.Name()); localPkg != nil {
			localVer = localPkg.Version()
		}

		slice = append(slice, upgrade{
			pkg.Name(),
			pkg.DB().Name(),
			localVer,
			pkg.Version(),
		})
		return nil
	})

	return slice, nil
}

// upgradePkgs handles updating the cache and installing updates.
func upgradePkgs(aurUp, repoUp upSlice) (ignore, aurNames stringset.StringSet, err error) {
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
	fmt.Printf("%s"+bold(" %d ")+"%s\n", bold(cyan("::")), allUpLen, bold(gotext.Get("Packages to upgrade.")))
	allUp.print()

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
