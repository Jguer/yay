package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	alpm "github.com/jguer/go-alpm"
	rpc "github.com/mikkeloscar/aur"
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

// Print prints the details of the packages to upgrade.
func (u upSlice) Print(start int) {
	for k, i := range u {
		old, errOld := pkgb.NewCompleteVersion(i.LocalVersion)
		new, errNew := pkgb.NewCompleteVersion(i.RemoteVersion)
		var left, right string

		f := func(name string) (color int) {
			var hash = 5381
			for i := 0; i < len(name); i++ {
				hash = int(name[i]) + ((hash << 5) + (hash))
			}
			return hash%6 + 31
		}
		fmt.Printf("\x1b[33m%-2d\x1b[0m ", len(u)+start-k-1)
		fmt.Printf("\x1b[1;%dm%s\x1b[0m/\x1b[1;39m%-25s\t\t\x1b[0m", f(i.Repository), i.Repository, i.Name)

		if errOld != nil {
			left = fmt.Sprintf("\x1b[31m%20s\x1b[0m", "Invalid Version")
		} else {
			left = fmt.Sprintf("\x1b[31m%18s\x1b[0m-%s", old.Version, old.Pkgrel)
		}

		if errNew != nil {
			right = fmt.Sprintf("\x1b[31m%s\x1b[0m", "Invalid Version")
		} else {
			right = fmt.Sprintf("\x1b[31m%s\x1b[0m-%s", new.Version, new.Pkgrel)
		}
		fmt.Printf("%s -> %s\n", left, right)
	}
}

// upList returns lists of packages to upgrade from each source.
func upList() (aurUp upSlice, repoUp upSlice, err error) {
	local, remote, _, remoteNames, err := filterPackages()
	if err != nil {
		return
	}

	repoC := make(chan upSlice)
	aurC := make(chan upSlice)
	errC := make(chan error)

	fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Searching databases for updates...\x1b[0m")
	go func() {
		repoUpList, err := upRepo(local)
		errC <- err
		repoC <- repoUpList
	}()

	fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Searching AUR for updates...\x1b[0m")
	go func() {
		aurUpList, err := upAUR(remote, remoteNames)
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

func isIgnored(name string, groups []string, oldVersion string, newVersion string) bool {
	for _, p := range alpmConf.IgnorePkg {
		if p == name {
			fmt.Printf("\x1b[33mwarning:\x1b[0m %s (ignored pkg) ignoring upgrade (%s -> %s)\n", name, oldVersion, newVersion)
			return true
		}
	}

	for _, g := range alpmConf.IgnoreGroup {
		for _, pg := range groups {
			if g == pg {
				fmt.Printf("\x1b[33mwarning:\x1b[0m %s (ignored pkg) ignoring upgrade (%s -> %s)\n", name, oldVersion, newVersion)
				return true
			}
		}

	}
	return false
}

func upDevel(remoteNames []string, packageC chan upgrade, done chan bool) {
	for _, e := range savedInfo {
		if e.needsUpdate() {
			found := false
			for _, r := range remoteNames {
				if r == e.Package {
					found = true
				}
			}
			if found && !isIgnored(e.Package, nil, e.SHA[0:6], "git") {
				packageC <- upgrade{e.Package, "devel", e.SHA[0:6], "git"}
			} else {
				removeVCSPackage([]string{e.Package})
			}
		}
	}
	done <- true
}

// upAUR gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, remoteNames []string) (toUpgrade upSlice, err error) {
	var j int
	var routines int
	var routineDone int

	packageC := make(chan upgrade)
	done := make(chan bool)

	if config.Devel {
		routines++
		go upDevel(remoteNames, packageC, done)
		fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Checking development packages...\x1b[0m")
	}

	for i := len(remote); i != 0; i = j {
		//Split requests so AUR RPC doesn't get mad at us.
		j = i - config.RequestSplitN
		if j < 0 {
			j = 0
		}

		routines++
		go func(local []alpm.Package, remote []string) {
			qtemp, err := rpc.Info(remote)
			if err != nil {
				fmt.Println(err)
				done <- true
				return
			}
			// For each item in query: Search equivalent in foreign.
			// We assume they're ordered and are returned ordered
			// and will only be missing if they don't exist in AUR.
			max := len(qtemp) - 1
			var missing, x int

			for i := range local {
				x = i - missing
				if x > max {
					break
				} else if qtemp[x].Name == local[i].Name() {
					if (config.TimeUpdate && (int64(qtemp[x].LastModified) > local[i].BuildDate().Unix())) ||
						(alpm.VerCmp(local[i].Version(), qtemp[x].Version) < 0) {
						if !isIgnored(local[i].Name(), local[i].Groups().Slice(), local[i].Version(), qtemp[x].Version) {
							packageC <- upgrade{qtemp[x].Name, "aur", local[i].Version(), qtemp[x].Version}
						}
					}
					continue
				} else {
					missing++
				}
			}
			done <- true
		}(remote[j:i], remoteNames[j:i])
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

		if newPkg != nil && !isIgnored(pkg.Name(), pkg.Groups().Slice(), pkg.Version(), newPkg.Version()) {
			slice = append(slice, upgrade{pkg.Name(), newPkg.DB().Name(), pkg.Version(), newPkg.Version()})
		}
	}
	return slice, nil
}

// upgradePkgs handles updating the cache and installing updates.
func upgradePkgs(flags []string) error {
	aurUp, repoUp, err := upList()
	if err != nil {
		return err
	} else if len(aurUp)+len(repoUp) == 0 {
		fmt.Println("\nthere is nothing to do")
		return err
	}

	var repoNums []int
	var aurNums []int
	sort.Sort(repoUp)
	fmt.Printf("\x1b[1;34;1m:: \x1b[0m\x1b[1m%d Packages to upgrade.\x1b[0m\n", len(aurUp)+len(repoUp))
	repoUp.Print(len(aurUp))
	aurUp.Print(0)

	if !config.NoConfirm {
		fmt.Print("\x1b[32mEnter packages you don't want to upgrade.\x1b[0m\nNumbers: ")
		reader := bufio.NewReader(os.Stdin)

		numberBuf, overflow, err := reader.ReadLine()
		if err != nil || overflow {
			fmt.Println(err)
			return err
		}

		result := strings.Fields(string(numberBuf))
		for _, numS := range result {
			num, err := strconv.Atoi(numS)
			if err != nil {
				continue
			}
			if num > len(aurUp)+len(repoUp)-1 || num < 0 {
				continue
			} else if num < len(aurUp) {
				num = len(aurUp) - num - 1
				aurNums = append(aurNums, num)
			} else {
				num = len(aurUp) + len(repoUp) - num - 1
				repoNums = append(repoNums, num)
			}
		}
	}

	if len(repoUp) != 0 {
		var repoNames []string
	repoloop:
		for i, k := range repoUp {
			for _, j := range repoNums {
				if j == i {
					continue repoloop
				}
			}
			repoNames = append(repoNames, k.Name)
		}
		
		arguments := makeArguments()
		arguments.addArg("S", "noconfirm")
		arguments.addArg(flags...)
		arguments.addTarget(repoNames...)
		
		err := passToPacman(arguments)
		if err != nil {
			fmt.Println("Error upgrading repo packages.")
		}
	}

	if len(aurUp) != 0 {
		var aurNames []string
	aurloop:
		for i, k := range aurUp {
			for _, j := range aurNums {
				if j == i {
					continue aurloop
				}
			}
			aurNames = append(aurNames, k.Name)
		}
		aurInstall(aurNames, flags)
	}
	return nil
}
