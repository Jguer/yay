package main

import (
	"fmt"
	"sort"
	"sync"
	"unicode"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v9/pkg/intrange"

	"github.com/Jguer/yay/v9/pkg/multierror"
	"github.com/Jguer/yay/v9/pkg/stringset"
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
		return LessRunes(iRunes, jRunes)
	}

	syncDB, err := alpmHandle.SyncDBs()
	if err != nil {
		iRunes := []rune(u[i].Repository)
		jRunes := []rune(u[j].Repository)
		return LessRunes(iRunes, jRunes)
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
	return LessRunes(iRunes, jRunes)

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

	return
}

// upList returns lists of packages to upgrade from each source.
func upList(warnings *aurWarnings) (upSlice, upSlice, error) {
	local, remote, _, remoteNames, err := filterPackages()
	if err != nil {
		return nil, nil, err
	}

	var wg sync.WaitGroup
	var develUp upSlice
	var repoUp upSlice
	var aurUp upSlice

	var errs multierror.MultiError

	aurdata := make(map[string]*rpc.Pkg)

	if mode == modeAny || mode == modeRepo {
		fmt.Println(bold(cyan("::") + bold(" Searching databases for updates...")))
		wg.Add(1)
		go func() {
			repoUp, err = upRepo(local)
			errs.Add(err)
			wg.Done()
		}()
	}

	if mode == modeAny || mode == modeAUR {
		fmt.Println(bold(cyan("::") + bold(" Searching AUR for updates...")))

		var _aurdata []*rpc.Pkg
		_aurdata, err = aurInfo(remoteNames, warnings)
		errs.Add(err)
		if err == nil {
			for _, pkg := range _aurdata {
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)
			go func() {
				aurUp, err = upAUR(remote, aurdata)
				errs.Add(err)
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

	err = alpmHandle.SyncSysupgrade(cmdArgs.existsDouble("u", "sysupgrade"))
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
func upgradePkgs(aurUp, repoUp upSlice) (stringset.StringSet, stringset.StringSet, error) {
	ignore := make(stringset.StringSet)
	aurNames := make(stringset.StringSet)

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
	fmt.Printf("%s"+bold(" %d ")+"%s\n", bold(cyan("::")), allUpLen, bold("Packages to upgrade."))
	allUp.print()

	fmt.Println(bold(green(arrow + " Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)")))
	fmt.Print(bold(green(arrow + " ")))

	numbers, err := getInput(config.AnswerUpgrade)
	if err != nil {
		return nil, nil, err
	}

	//upgrade menu asks you which packages to NOT upgrade so in this case
	//include and exclude are kind of swapped
	//include, exclude, other := parseNumberMenu(string(numberBuf))
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
