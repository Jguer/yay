package pacman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jguer/go-alpm"
	"github.com/jguer/yay/config"
	"github.com/jguer/yay/util"
)

// Query describes a Repository search.
type Query []Result

// Result describes a pkg.
type Result struct {
	Name        string
	Repository  string
	Version     string
	Description string
	Group       string
	Installed   bool
}

// UpdatePackages handles cache update and upgrade
func UpdatePackages(flags []string) error {
	args := append([]string{"pacman", "-Syu"}, flags...)

	cmd := exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	return err
}

// Search handles repo searches. Creates a RepoSearch struct.
func Search(pkgInputN []string) (s Query, n int, err error) {
	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	var installed bool
	dbS := dbList.Slice()

	// BottomUp functions
	initL := func(len int) int {
		return len - 1
	}

	compL := func(len int, i int) bool {
		return i > 0
	}

	finalL := func(i int) int {
		return i - 1
	}

	// TopDown functions
	if util.SortMode == util.TopDown {
		initL = func(len int) int {
			return 0
		}

		compL = func(len int, i int) bool {
			return i < len
		}

		finalL = func(i int) int {
			return i + 1
		}
	}

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
				installed = false
				if r, _ := localDb.PkgByName(pkgS[i].Name()); r != nil {
					installed = true
				}
				n++

				s = append(s, Result{
					Name:        pkgS[i].Name(),
					Description: pkgS[i].Description(),
					Version:     pkgS[i].Version(),
					Repository:  dbS[f].Name(),
					Group:       strings.Join(pkgS[i].Groups().Slice(), ","),
					Installed:   installed,
				})
			}
		}
	}
	return
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s Query) PrintSearch() {
	for i, res := range s {
		var toprint string
		if util.SearchVerbosity == util.NumberMenu {
			if util.SortMode == util.BottomUp {
				toprint += fmt.Sprintf("%d ", len(s)-i-1)
			} else {
				toprint += fmt.Sprintf("%d ", i)
			}
		} else if util.SearchVerbosity == util.Minimal {
			fmt.Println(res.Name)
			continue
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m",
			res.Repository, res.Name, res.Version)

		if len(res.Group) != 0 {
			toprint += fmt.Sprintf("(%s) ", res.Group)
		}

		if res.Installed {
			toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
		}

		toprint += "\n" + res.Description
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
		for _, db := range dbList.Slice() {
			_, err = db.PkgByName(pkg)
			if err == nil {
				found = true
				repo = append(repo, pkg)
				break
			}
		}

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

// BuildDependencies finds packages, on the second run
// compares with a baselist and avoids searching those
func BuildDependencies(baselist []string) func(toCheck []string, isBaseList bool, last bool) (repo []string, notFound []string) {
	localDb, _ := config.AlpmHandle.LocalDb()
	dbList, _ := config.AlpmHandle.SyncDbs()

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

// Install sends an install command to pacman with the pkgName slice
func Install(pkgName []string, flags []string) (err error) {
	if len(pkgName) == 0 {
		return nil
	}

	args := []string{"pacman", "-S"}
	args = append(args, pkgName...)
	args = append(args, flags...)

	cmd := exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	return nil
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func CleanRemove(pkgName []string) (err error) {
	if len(pkgName) == 0 {
		return nil
	}

	args := []string{"pacman", "-Rnsc"}
	args = append(args, pkgName...)
	args = append(args, "--noutil.Conf.rm")

	cmd := exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	return nil
}

// ForeignPackages returns a map of foreign packages, with their version and date as values.
func ForeignPackages() (foreign map[string]*struct {
	Version string
	Date    int64
}, n int, err error) {

	localDb, err := config.AlpmHandle.LocalDb()
	if err != nil {
		return
	}
	dbList, err := config.AlpmHandle.SyncDbs()
	if err != nil {
		return
	}

	foreign = make(map[string]*struct {
		Version string
		Date    int64
	})
	// Find foreign packages in system
	for _, pkg := range localDb.PkgCache().Slice() {
		// Change to more effective method
		found := false
		for _, db := range dbList.Slice() {
			_, err = db.PkgByName(pkg.Name())
			if err == nil {
				found = true
				break
			}
		}

		if !found {
			foreign[pkg.Name()] = &struct {
				Version string
				Date    int64
			}{pkg.Version(), pkg.InstallDate().Unix()}
			n++
		}
	}

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
		fmt.Printf("%s: \x1B[0;33m%dMB\x1B[0m\n", pkgS[i].Name(), pkgS[i].ISize()/(1024*1024))
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
			fmt.Printf("%s: \x1B[0;33m%dMB\x1B[0m\n", pkg.Name(), pkg.ISize()/(1024*1024))

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
				fmt.Printf("%s: \x1B[0;33m%dMB\x1B[0m\n", pkg.Name(), pkg.ISize()/(1024*1024))
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

	p := func(pkg alpm.Package) error {
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
	}

	f := func(db alpm.Db) error {
		db.PkgCache().ForEach(p)
		return nil
	}

	dbList.ForEach(f)
	return nil
}
