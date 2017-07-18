// Package upgrade package is responsible for returning lists of outdated packages.
package upgrade

import (
	"fmt"
	"unicode"

	alpm "github.com/jguer/go-alpm"
	"github.com/jguer/yay/config"
	rpc "github.com/mikkeloscar/aur"
	pkgb "github.com/mikkeloscar/gopkgbuild"
)

// Upgrade type describes a system upgrade.
type Upgrade struct {
	Name          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
}

// Slice is a slice of Upgrades
type Slice []Upgrade

func (s Slice) Len() int      { return len(s) }
func (s Slice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func (s Slice) Less(i, j int) bool {
	iRunes := []rune(s[i].Repository)
	jRunes := []rune(s[j].Repository)

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
func Print(start int, u Slice) {
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
			return (hash)%6 + 31
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
func List() (aurUp Slice, repoUp Slice, err error) {
	err = config.PassToPacman("-Sy", nil, nil)
	if err != nil {
		return
	}

	local, remote, _, remoteNames, err := FilterPackages()
	if err != nil {
		return
	}

	repoC := make(chan []Upgrade)
	aurC := make(chan []Upgrade)
	errC := make(chan error)

	go func() {
		repoUpList, err := repo(local)
		errC <- err
		repoC <- repoUpList
	}()

	go func() {
		aurUpList, err := aur(remote, remoteNames)
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
func aur(remote []alpm.Package, remoteNames []string) (toUpgrade Slice, err error) {
	var j int
	var routines int
	var routineDone int

	packageC := make(chan Upgrade)
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
						packageC <- Upgrade{qtemp[x].Name, "aur", local[i].Version(), qtemp[x].Version}
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
func repo(local []alpm.Package) (Slice, error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	slice := Slice{}
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

			slice = append(slice, Upgrade{pkg.Name(), newPkg.DB().Name(), pkg.Version(), newPkg.Version()})
		}
	}
	return slice, nil
}
