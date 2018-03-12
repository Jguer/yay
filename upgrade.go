package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"unicode"

	alpm "github.com/jguer/go-alpm"
	pkgb "github.com/mikkeloscar/gopkgbuild"
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
	iRunes := []rune(u[i].Repository)
	jRunes := []rune(u[j].Repository)

	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir > ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir > jr
		}
	}

	return false
}

func getVersionDiff(oldVersion, newversion string) (left, right string) {
	old, errOld := pkgb.NewCompleteVersion(oldVersion)
	new, errNew := pkgb.NewCompleteVersion(newversion)

	if errOld != nil {
		left = red("Invalid Version")
	}
	if errNew != nil {
		right = red("Invalid Version")
	}

	if errOld == nil && errNew == nil {
		if old.Version == new.Version {
			left = string(old.Version) + "-" + red(string(old.Pkgrel))
			right = string(new.Version) + "-" + green(string(new.Pkgrel))
		} else {
			left = red(string(old.Version)) + "-" + string(old.Pkgrel)
			right = bold(green(string(new.Version))) + "-" + string(new.Pkgrel)
		}
	}

	return
}

// upList returns lists of packages to upgrade from each source.
func upList(dt *depTree) (aurUp upSlice, repoUp upSlice, err error) {
	local, remote, _, remoteNames, err := filterPackages()
	if err != nil {
		return
	}

	repoC := make(chan upSlice)
	aurC := make(chan upSlice)
	errC := make(chan error)

	fmt.Println(bold(cyan("::") + " Searching databases for updates..."))
	go func() {
		repoUpList, err := upRepo(local)
		errC <- err
		repoC <- repoUpList
	}()

	fmt.Println(bold(cyan("::") + " Searching AUR for updates..."))
	go func() {
		aurUpList, err := upAUR(remote, remoteNames, dt)
		errC <- err
		aurC <- aurUpList
	}()

	var i = 0
loop:
	for {
		select {
		case repoUp = <-repoC:
			i++
		case aurUp = <-aurC:
			i++
		case err := <-errC:
			if err != nil {
				fmt.Println(err)
			}
		default:
			if i == 2 {
				close(repoC)
				close(aurC)
				close(errC)
				break loop
			}
		}
	}
	return
}

func upDevel(remote []alpm.Package, packageC chan upgrade, done chan bool) {
	for vcsName, e := range savedInfo {
		if e.needsUpdate() {
			found := false
			var pkg alpm.Package
			for _, r := range remote {
				if r.Name() == vcsName {
					found = true
					pkg = r
				}
			}
			if found {
				if pkg.ShouldIgnore() {
					left, right := getVersionDiff(pkg.Version(), "latest-commit")
					fmt.Print(magenta("Warning: "))
					fmt.Printf("%s ignoring package upgrade (%s => %s)\n", cyan(pkg.Name()), left, right)
				} else {
					packageC <- upgrade{pkg.Name(), "devel", pkg.Version(), "latest-commit"}
				}
			} else {
				removeVCSPackage([]string{vcsName})
			}
		}
	}
	done <- true
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, remoteNames []string, dt *depTree) (toUpgrade upSlice, err error) {
	var routines int
	var routineDone int

	packageC := make(chan upgrade)
	done := make(chan bool)

	if config.Devel {
		routines++
		go upDevel(remote, packageC, done)
		fmt.Println(bold(cyan("::") + " Checking development packages..."))
	}

	routines++
	go func(remote []alpm.Package, remoteNames []string, dt *depTree) {
		for _, pkg := range remote {
			aurPkg, ok := dt.Aur[pkg.Name()]
			if !ok {
				continue
			}

			if (config.TimeUpdate && (int64(aurPkg.LastModified) > pkg.BuildDate().Unix())) ||
				(alpm.VerCmp(pkg.Version(), aurPkg.Version) < 0) {
				if pkg.ShouldIgnore() {
					left, right := getVersionDiff(pkg.Version(), aurPkg.Version)
					fmt.Print(magenta("Warning: "))
					fmt.Printf("%s ignoring package upgrade (%s => %s)\n", cyan(pkg.Name()), left, right)
				} else {
					packageC <- upgrade{aurPkg.Name, "aur", pkg.Version(), aurPkg.Version}
				}
			}
		}

		done <- true
	}(remote, remoteNames, dt)

	if routineDone == routines {
		err = nil
		return
	}

	for {
		select {
		case pkg := <-packageC:
			for _, w := range toUpgrade {
				if w.Name == pkg.Name {
					continue
				}
			}
			toUpgrade = append(toUpgrade, pkg)
		case <-done:
			routineDone++
			if routineDone == routines {
				err = nil
				return
			}
		}
	}
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func upRepo(local []alpm.Package) (upSlice, error) {
	dbList, err := alpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	slice := upSlice{}

	for _, pkg := range local {
		newPkg := pkg.NewVersion(dbList)
		if newPkg != nil {
			if pkg.ShouldIgnore() {
				left, right := getVersionDiff(pkg.Version(), newPkg.Version())
				fmt.Print(magenta("Warning: "))
				fmt.Printf("%s ignoring package upgrade (%s => %s)\n", cyan(pkg.Name()), left, right)
			} else {
				slice = append(slice, upgrade{pkg.Name(), newPkg.DB().Name(), pkg.Version(), newPkg.Version()})
			}
		}
	}
	return slice, nil
}

//Contains returns whether e is present in s
func containsInt(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// RemoveIntListFromList removes all src's elements that are present in target
func removeIntListFromList(src, target []int) []int {
	max := len(target)
	for i := 0; i < max; i++ {
		if containsInt(src, target[i]) {
			target = append(target[:i], target[i+1:]...)
			max--
			i--
		}
	}
	return target
}

// upgradePkgs handles updating the cache and installing updates.
func upgradePkgs(dt *depTree) (stringSet, stringSet, error) {
	repoNames := make(stringSet)
	aurNames := make(stringSet)

	aurUp, repoUp, err := upList(dt)
	if err != nil {
		return repoNames, aurNames, err
	} else if len(aurUp)+len(repoUp) == 0 {
		return repoNames, aurNames, err
	}

	sort.Sort(repoUp)
	fmt.Println(bold(blue("::")), len(aurUp)+len(repoUp), bold("Packages to upgrade."))
	repoUp.Print(len(aurUp) + 1)
	aurUp.Print(1)

	if config.NoConfirm {
		for _, up := range repoUp {
			repoNames.set(up.Name)
		}
		for _, up := range aurUp {
			aurNames.set(up.Name)
		}
		return repoNames, aurNames, nil
	}

	fmt.Println(bold(green(arrow + " Packages to not upgrade (eg: 1 2 3, 1-3, ^4 or repo name)")))
	fmt.Print(bold(green(arrow + " ")))
	reader := bufio.NewReader(os.Stdin)

	numberBuf, overflow, err := reader.ReadLine()
	if err != nil {
		return nil, nil, err
	}

	if overflow {
		return nil, nil, fmt.Errorf("Input too long")
	}

	//upgrade menu asks you which packages to NOT upgrade so in this case
	//include and exclude are kind of swaped
	//include, exclude, other := parseNumberMenu(string(numberBuf))
	include, exclude, otherInclude, otherExclude := parseNumberMenu(string(numberBuf))

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoUp {
		if isInclude && otherInclude.get(pkg.Repository) {
			continue
		}

		if isInclude && !include.get(len(repoUp)-i+len(aurUp)) {
			repoNames.set(pkg.Name)
		}

		if !isInclude && (exclude.get(len(repoUp)-i+len(aurUp)) || otherExclude.get(pkg.Repository)) {
			repoNames.set(pkg.Name)
		}
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

	return repoNames, aurNames, err
}
