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
	"github.com/jguer/yay/aur"
	"github.com/jguer/yay/config"
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

// Slice is a slice of Upgrades
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

// FilterPackages filters packages based on source and type.
func FilterPackages() (local []alpm.Package, remote []alpm.Package,
	localNames []string, remoteNames []string, err error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	f := func(k alpm.Package) error {
		found := false
		// For each DB search for our secret package.
		_ = dbList.ForEach(func(d alpm.Db) error {
			if found {
				return nil
			}
			_, err := d.PkgByName(k.Name())
			if err == nil {
				found = true
				local = append(local, k)
				localNames = append(localNames, k.Name())
			}
			return nil
		})

		if !found {
			remote = append(remote, k)
			remoteNames = append(remoteNames, k.Name())
		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// Print prints the details of the packages to upgrade.
func (u upSlice) Print(start int) {
	for k, i := range u {
		old, err := pkgb.NewCompleteVersion(i.LocalVersion)
		if err != nil {
			fmt.Println(i.Name, err)
		}
		new, err := pkgb.NewCompleteVersion(i.RemoteVersion)
		if err != nil {
			fmt.Println(i.Name, err)
		}

		f := func(name string) (color int) {
			var hash = 5381
			for i := 0; i < len(name); i++ {
				hash = int(name[i]) + ((hash << 5) + (hash))
			}
			return hash%6 + 31
		}
		fmt.Printf("\x1b[33m%-2d\x1b[0m ", len(u)+start-k-1)
		fmt.Printf("\x1b[1;%dm%s\x1b[0m/\x1b[1;39m%-25s\t\t\x1b[0m", f(i.Repository), i.Repository, i.Name)

		if old.Version != new.Version {
			fmt.Printf("\x1b[31m%18s\x1b[0m-%d -> \x1b[1;32m%s\x1b[0m-%d\x1b[0m",
				old.Version, old.Pkgrel,
				new.Version, new.Pkgrel)
		} else {
			fmt.Printf("\x1b[0m%18s-\x1b[31m%d\x1b[0m -> %s-\x1b[32m%d\x1b[0m",
				old.Version, old.Pkgrel,
				new.Version, new.Pkgrel)
		}
		print("\n")
	}
}

// List returns lists of packages to upgrade from each source.
func upList() (aurUp upSlice, repoUp upSlice, err error) {
	local, remote, _, remoteNames, err := FilterPackages()
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

// aur gathers foreign packages and checks if they have new versions.
// Output: Upgrade type package list.
func upAUR(remote []alpm.Package, remoteNames []string) (toUpgrade upSlice, err error) {
	var j int
	var routines int
	var routineDone int

	packageC := make(chan upgrade)
	done := make(chan bool)

	for i := len(remote); i != 0; i = j {
		//Split requests so AUR RPC doesn't get mad at us.
		j = i - config.YayConf.RequestSplitN
		if j < 0 {
			j = 0
		}

		routines++
		go func(local []alpm.Package, remote []string) {
			qtemp, err := rpc.Info(remoteNames)
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
					if (config.YayConf.TimeUpdate && (int64(qtemp[x].LastModified) > local[i].BuildDate().Unix())) ||
						(alpm.VerCmp(local[i].Version(), qtemp[x].Version) < 0) {
						packageC <- upgrade{qtemp[x].Name, "aur", local[i].Version(), qtemp[x].Version}
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

// repo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func upRepo(local []alpm.Package) (upSlice, error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	slice := upSlice{}
primeloop:
	for _, pkg := range local {
		newPkg := pkg.NewVersion(dbList)

		if newPkg != nil {
			for _, ignorePkg := range config.AlpmConf.IgnorePkg {
				if pkg.Name() == ignorePkg {
					fmt.Printf("\x1b[33mwarning:\x1b[0m %s (ignored pkg) ignoring upgrade (%s -> %s)\n", pkg.Name(), pkg.Version(), newPkg.Version())
					continue primeloop
				}
			}

			for _, ignoreGroup := range config.AlpmConf.IgnoreGroup {
				for _, group := range pkg.Groups().Slice() {
					if group == ignoreGroup {
						fmt.Printf("\x1b[33mwarning:\x1b[0m %s (ignored group) ignoring upgrade (%s -> %s)\n", pkg.Name(), pkg.Version(), newPkg.Version())
						continue primeloop

					}
				}
			}

			slice = append(slice, upgrade{pkg.Name(), newPkg.DB().Name(), pkg.Version(), newPkg.Version()})
		}
	}
	return slice, nil
}

// Upgrade handles updating the cache and installing updates.
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

	if !config.YayConf.NoConfirm {
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

		err := config.PassToPacman("-S", repoNames, append(flags, "--noconfirm"))
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
		aur.Install(aurNames, flags)
	}
	return nil
}
