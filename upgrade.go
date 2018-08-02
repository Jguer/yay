package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
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
		return lessRunes(iRunes, jRunes)
	}

	syncDb, err := alpmHandle.SyncDbs()
	if err != nil {
		iRunes := []rune(u[i].Repository)
		jRunes := []rune(u[j].Repository)
		return lessRunes(iRunes, jRunes)
	}

	less := false
	found := syncDb.ForEach(func(db alpm.Db) error {
		if db.Name() == u[i].Repository {
			less = true
		} else if db.Name() == u[j].Repository {
			less = false
		} else {
			return nil
		}

		return fmt.Errorf("")
	})

	if found != nil {
		return less
	}

	iRunes := []rune(u[i].Repository)
	jRunes := []rune(u[j].Repository)
	return lessRunes(iRunes, jRunes)

}

func getVersionDiff(oldVersion, newVersion string) (left, right string) {
	if oldVersion == newVersion {
		return oldVersion, newVersion
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

	return
}

// upList returns lists of packages to upgrade from each source.
func upList(warnings *aurWarnings) (aurUp upSlice, repoUp upSlice, err error) {
	local, remote, _, remoteNames, err := filterPackages()
	if err != nil {
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var develUp upSlice

	var repoErr error
	var aurErr error

	aurdata := make(map[string]*rpc.Pkg)

	if mode == ModeAny || mode == ModeRepo {
		fmt.Println(bold(cyan("::") + bold(" Searching databases for updates...")))
		wg.Add(1)
		go func() {
			repoUp, repoErr = upRepo(local)
			wg.Done()
		}()
	}

	if mode == ModeAny || mode == ModeAUR {
		fmt.Println(bold(cyan("::") + bold(" Searching AUR for updates...")))

		var _aurdata []*rpc.Pkg
		_aurdata, aurErr = aurInfo(remoteNames, warnings)
		if aurErr == nil {
			for _, pkg := range _aurdata {
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)
			go func() {
				aurUp, aurErr = upAUR(remote, aurdata)
				wg.Done()
			}()

			if config.Devel {
				fmt.Println(bold(cyan("::") + bold(" Checking development packages...")))
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

	errs := make([]string, 0)
	for _, e := range []error{repoErr, aurErr} {
		if e != nil {
			errs = append(errs, e.Error())
		}
	}

	if len(errs) > 0 {
		err = fmt.Errorf("%s", strings.Join(errs, "\n"))
		return nil, nil, err
	}

	if develUp != nil {
		names := make(stringSet)
		for _, up := range develUp {
			names.set(up.Name)
		}
		for _, up := range aurUp {
			if !names.get(up.Name) {
				develUp = append(develUp, up)
			}
		}

		aurUp = develUp
	}

	return aurUp, repoUp, err
}

func upDevel(remote []alpm.Package, aurdata map[string]*rpc.Pkg) (toUpgrade upSlice) {
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

	for _, pkg := range toUpdate {
		if pkg.ShouldIgnore() {
			printIgnoringPackage(pkg, "latest-commit")
		} else {
			toUpgrade = append(toUpgrade, upgrade{pkg.Name(), "devel", pkg.Version(), "latest-commit"})
		}
	}

	removeVCSPackage(toRemove)
	return
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, aurdata map[string]*rpc.Pkg) (upSlice, error) {
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

	return toUpgrade, nil
}

func printIgnoringPackage(pkg alpm.Package, newPkgVersion string) {
	left, right := getVersionDiff(pkg.Version(), newPkgVersion)

	fmt.Printf("%s %s: ignoring package upgrade (%s => %s)\n",
		yellow(bold(smallArrow)),
		cyan(pkg.Name()),
		left, right,
	)
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
					yellow(bold(smallArrow)),
					cyan(pkg.Name()),
					left, right,
				)
		}
	}
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func upRepo(local []alpm.Package) (upSlice, error) {
	slice := upSlice{}

	localDB, err := alpmHandle.LocalDb()
	if err != nil {
		return slice, err
	}

	err = alpmHandle.TransInit(alpm.TransFlagNoLock)
	if err != nil {
		return slice, err
	}

	defer alpmHandle.TransRelease()

	alpmHandle.SyncSysupgrade(cmdArgs.existsDouble("u", "sysupgrade"))
	alpmHandle.TransGetAdd().ForEach(func(pkg alpm.Package) error {
		localPkg, err := localDB.PkgByName(pkg.Name())
		localVer := "-"

		if err == nil {
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
func upgradePkgs(aurUp, repoUp upSlice) (stringSet, stringSet, error) {
	ignore := make(stringSet)
	aurNames := make(stringSet)

	allUpLen := len(repoUp) + len(aurUp)
	if allUpLen == 0 {
		return ignore, aurNames, nil
	}

	if !config.UpgradeMenu {
		for _, pkg := range aurUp {
			aurNames.set(pkg.Name)
		}

		return ignore, aurNames, nil
	}

	sort.Sort(repoUp)
	sort.Sort(aurUp)
	allUp := append(repoUp, aurUp...)
	fmt.Printf("%s"+bold(" %d ")+"%s\n", bold(cyan("::")), allUpLen, bold("Packages to upgrade."))
	allUp.print()

	fmt.Println(bold(green(arrow + " Packages to not upgrade: (eg: 1 2 3, 1-3, ^4 or repo name)")))
	fmt.Print(bold(green(arrow + " ")))

	numbers, err := getInput(config.AnswerUpgrade)
	if err != nil {
		return nil, nil, err
	}

	//upgrade menu asks you which packages to NOT upgrade so in this case
	//include and exclude are kind of swapped
	//include, exclude, other := parseNumberMenu(string(numberBuf))
	include, exclude, otherInclude, otherExclude := parseNumberMenu(numbers)

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoUp {
		if isInclude && otherInclude.get(pkg.Repository) {
			ignore.set(pkg.Name)
		}

		if isInclude && !include.get(len(repoUp)-i+len(aurUp)) {
			continue
		}

		if !isInclude && (exclude.get(len(repoUp)-i+len(aurUp)) || otherExclude.get(pkg.Repository)) {
			continue
		}

		ignore.set(pkg.Name)
	}

	for i, pkg := range aurUp {
		if isInclude && otherInclude.get(pkg.Repository) {
			continue
		}

		if isInclude && !include.get(len(aurUp)-i) {
			aurNames.set(pkg.Name)
		}

		if !isInclude && (exclude.get(len(aurUp)-i) || otherExclude.get(pkg.Repository)) {
			aurNames.set(pkg.Name)
		}
	}

	return ignore, aurNames, err
}
