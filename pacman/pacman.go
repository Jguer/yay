package pacman

import (
	"fmt"
	"os"
	"strings"

	"github.com/jguer/go-alpm"
	"github.com/jguer/yay/config"
)

// Query holds the results of a repository search.
type Query []alpm.Package

// Search handles repo searches. Creates a RepoSearch struct.
func Search(pkgInputN []string) (s Query, n int, err error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	// BottomUp functions
	initL := func(len int) int {
		if config.YayConf.SortMode == config.TopDown {
			return 0
		} else {
			return len - 1
		}
	}
	compL := func(len int, i int) bool {
		if config.YayConf.SortMode == config.TopDown {
			return i < len
		} else {
			return i > -1
		}
	}
	finalL := func(i int) int {
		if config.YayConf.SortMode == config.TopDown {
			return i + 1
		} else {
			return i - 1
		}
	}

	dbS := dbList.Slice()
	lenDbs := len(dbS)
	for f := initL(lenDbs); compL(lenDbs, f); f = finalL(f) {
		pkgS := dbS[f].PkgCache().Slice()
		lenPkgs := len(pkgS)
		for i := initL(lenPkgs); compL(lenPkgs, i); i = finalL(i) {
			match := true
			for _, pkgN := range pkgInputN {
				if !(strings.Contains(pkgS[i].Name(), pkgN) || strings.Contains(strings.ToLower(pkgS[i].Description()), pkgN)) {
					match = false
					break
				}
			}

			if match {
				n++
				s = append(s, pkgS[i])
			}
		}
	}
	return
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s Query) PrintSearch() {
	for i, res := range s {
		var toprint string
		if config.YayConf.SearchMode == config.NumberMenu {
			if config.YayConf.SortMode == config.BottomUp {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", len(s)-i-1)
			} else {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", i)
			}
		} else if config.YayConf.SearchMode == config.Minimal {
			fmt.Println(res.Name())
			continue
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m",
			res.DB().Name(), res.Name(), res.Version())

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := config.AlpmHandle.LocalDb()
		if err == nil {
			if _, err = localDb.PkgByName(res.Name()); err == nil {
				toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// PackageSlices separates an input slice into aur and repo slices
func PackageSlices(toCheck []string) (aur []string, repo []string, err error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	for _, pkg := range toCheck {
		found := false

		_ = dbList.ForEach(func(db alpm.Db) error {
			if found {
				return nil
			}

			_, err = db.PkgByName(pkg)
			if err == nil {
				found = true
				repo = append(repo, pkg)
			}
			return nil
		})

		if !found {
			if _, errdb := dbList.PkgCachebyGroup(pkg); errdb == nil {
				repo = append(repo, pkg)
			} else {
				aur = append(aur, pkg)
			}
		}
	}

	err = nil
	return
}

func UpgradeList() ([]alpm.Package, error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return nil, err
	}

	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return nil, err
	}

	slice := []alpm.Package{}
	for _, pkg := range localDb.PkgCache().Slice() {
		newPkg := pkg.NewVersion(dbList)
		if newPkg != nil {
			slice = append(slice, *newPkg)
		}
	}
	return slice, nil
}

// BuildDependencies finds packages, on the second run
// compares with a baselist and avoids searching those
func BuildDependencies(baselist []string) func(toCheck []string, isBaseList bool, last bool) (repo []string, notFound []string) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		panic(err)
	}

	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		panic(err)
	}

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}

	return func(toCheck []string, isBaseList bool, close bool) (repo []string, notFound []string) {
		if close {
			return
		}

	Loop:
		for _, dep := range toCheck {
			if !isBaseList {
				for _, base := range baselist {
					if base == dep {
						continue Loop
					}
				}
			}
			if _, erp := localDb.PkgCache().FindSatisfier(dep); erp == nil {
				continue
			} else if pkg, erp := dbList.FindSatisfier(dep); erp == nil {
				repo = append(repo, pkg.Name())
			} else {
				field := strings.FieldsFunc(dep, f)
				notFound = append(notFound, field[0])
			}
		}
		return
	}
}

// DepSatisfier receives a string slice, returns a slice of packages found in
// repos and one of packages not found in repos. Leaves out installed packages.
func DepSatisfier(toCheck []string) (repo []string, notFound []string, err error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}

	for _, dep := range toCheck {
		if _, erp := localDb.PkgCache().FindSatisfier(dep); erp == nil {
			continue
		} else if pkg, erp := dbList.FindSatisfier(dep); erp == nil {
			repo = append(repo, pkg.Name())
		} else {
			field := strings.FieldsFunc(dep, f)
			notFound = append(notFound, field[0])
		}
	}

	err = nil
	return
}

// PkgNameSlice returns a slice of package names
// func (s Query) PkgNameSlice() (pkgNames []string) {
// 	for _, e := range s {
// 		pkgNames = append(pkgNames, e.Name())
// 	}
// 	return
// }

// CleanRemove sends a full removal command to pacman with the pkgName slice
func CleanRemove(pkgName []string) (err error) {
	if len(pkgName) == 0 {
		return nil
	}

	err = config.PassToPacman("-Rsnc", pkgName, []string{"--noconfirm"})
	return err
}

// ForeignPackages returns a map of foreign packages, with their version and date as values.
func ForeignPackages() (foreign map[string]alpm.Package, err error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	foreign = make(map[string]alpm.Package)

	f := func(k alpm.Package) error {
		found := false
		_ = dbList.ForEach(func(d alpm.Db) error {
			if found {
				return nil
			}
			_, err = d.PkgByName(k.Name())
			if err == nil {
				found = true
			}
			return nil
		})

		if !found {
			foreign[k.Name()] = k
		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// ForeignPackages returns a map of foreign packages, with their version and date as values.
func ForeignPackageList() (packages []alpm.Package, packageNames []string, err error) {
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
		_ = dbList.ForEach(func(d alpm.Db) error {
			if found {
				return nil
			}
			_, err = d.PkgByName(k.Name())
			if err == nil {
				found = true
			}
			return nil
		})

		if !found {
			packages = append(packages, k)
			packageNames = append(packageNames, k.Name())
		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// Statistics returns statistics about packages installed in system
func Statistics() (info struct {
	Totaln    int
	Expln     int
	TotalSize int64
}, err error) {
	var tS int64 // TotalSize
	var nPkg int
	var ePkg int

	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}

	for _, pkg := range localDb.PkgCache().Slice() {
		tS += pkg.ISize()
		nPkg++
		if pkg.Reason() == 0 {
			ePkg++
		}
	}

	info = struct {
		Totaln    int
		Expln     int
		TotalSize int64
	}{
		nPkg, ePkg, tS,
	}

	return
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func BiggestPackages() {

	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}

	pkgCache := localDb.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkgS[i].Name(), config.Human(pkgS[i].ISize()))
	}
	// Could implement size here as well, but we just want the general idea
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
func HangingPackages() (hanging []string, err error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}

	f := func(pkg alpm.Package) error {
		if pkg.Reason() != alpm.PkgReasonDepend {
			return nil
		}
		requiredby := pkg.ComputeRequiredBy()
		if len(requiredby) == 0 {
			hanging = append(hanging, pkg.Name())
			fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), config.Human(pkg.ISize()))

		}
		return nil
	}

	err = localDb.PkgCache().ForEach(f)
	return
}

// SliceHangingPackages returns a list of packages installed as deps
// and unneeded by the system from a provided list of package names.
func SliceHangingPackages(pkgS []string) (hanging []string) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}

big:
	for _, pkgName := range pkgS {
		for _, hangN := range hanging {
			if hangN == pkgName {
				continue big
			}
		}

		pkg, err := localDb.PkgByName(pkgName)
		if err == nil {
			if pkg.Reason() != alpm.PkgReasonDepend {
				continue
			}

			requiredby := pkg.ComputeRequiredBy()
			if len(requiredby) == 0 {
				hanging = append(hanging, pkgName)
				fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), config.Human(pkg.ISize()))
			}
		}
	}
	return
}

// GetPkgbuild downloads pkgbuild from the ABS.
func GetPkgbuild(pkgN string, path string) (err error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	for _, db := range dbList.Slice() {
		pkg, err := db.PkgByName(pkgN)
		if err == nil {
			var url string
			if db.Name() == "core" || db.Name() == "extra" {
				url = "https://projects.archlinux.org/svntogit/packages.git/snapshot/packages/" + pkg.Name() + ".tar.gz"
			} else if db.Name() == "community" {
				url = "https://projects.archlinux.org/svntogit/community.git/snapshot/community-packages/" + pkg.Name() + ".tar.gz"
			} else {
				return fmt.Errorf("Not in standard repositories")
			}
			fmt.Printf("\x1b[1;32m==>\x1b[1;33m %s \x1b[1;32mfound in ABS.\x1b[0m\n", pkgN)
			errD := config.DownloadAndUnpack(url, path, true)
			return errD
		}
	}
	return fmt.Errorf("package not found")
}

//CreatePackageList appends Repo packages to completion cache
func CreatePackageList(out *os.File) (err error) {
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	_ = dbList.ForEach(func(db alpm.Db) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			fmt.Print(pkg.Name())
			out.WriteString(pkg.Name())
			if config.YayConf.Shell == "fish" {
				fmt.Print("\t" + pkg.DB().Name() + "\n")
				out.WriteString("\t" + pkg.DB().Name() + "\n")
			} else {
				fmt.Print("\n")
				out.WriteString("\n")
			}
			return nil
		})
		return nil
	})
	return nil
}
