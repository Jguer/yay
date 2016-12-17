package pacman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jguer/go-alpm"
)

// RepoSearch describes a Repository search.
type RepoSearch []Result

// Result describes a pkg.
type Result struct {
	Name        string
	Repository  string
	Version     string
	Description string
	Group       string
	Installed   bool
}

// PacmanConf describes the default pacman config file
const PacmanConf string = "/etc/pacman.conf"

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode NumberMenu and Search
var SortMode = DownTop

// Determines NumberMenu and Search Order
const (
	DownTop = iota
	TopDown
)

var conf alpm.PacmanConfig

func init() {
	conf, _ = readConfig(PacmanConf)
}

func readConfig(pacmanconf string) (conf alpm.PacmanConfig, err error) {
	file, err := os.Open(pacmanconf)
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}
	return
}

// UpdatePackages handles cache update and upgrade
func UpdatePackages(flags []string) error {
	var cmd *exec.Cmd
	var args []string

	args = append(args, "pacman", "-Syu")
	args = append(args, flags...)

	cmd = exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

// Search handles repo searches. Creates a RepoSearch struct.
func Search(pkgName string) (s RepoSearch, n int, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return
	}
	dbList, err := h.SyncDbs()
	if err != nil {
		return
	}

	var installed bool
	dbS := dbList.Slice()
	var f int
	if SortMode == DownTop {
		f = len(dbS) - 1
	} else {
		f = 0
	}

	for {
		pkgS := dbS[f].PkgCache().Slice()

		var i int
		if SortMode == DownTop {
			i = len(pkgS) - 1
		} else {
			i = 0
		}

		for {
			if strings.Contains(pkgS[i].Name(), pkgName) {
				if r, _ := localDb.PkgByName(pkgS[i].Name()); r != nil {
					installed = true
				} else {
					installed = false
				}

				s = append(s, Result{
					Name:        pkgS[i].Name(),
					Description: pkgS[i].Description(),
					Version:     pkgS[i].Version(),
					Repository:  dbS[f].Name(),
					Group:       strings.Join(pkgS[i].Groups().Slice(), ","),
					Installed:   installed,
				})
				n++
			}

			if SortMode == DownTop {
				if i > 0 {
					i--
				} else {
					break
				}
			} else {
				if i < len(pkgS)-1 {
					i++
				} else {
					break
				}
			}
		}

		if SortMode == DownTop {
			if f > 0 {
				f--
			} else {
				break
			}
		} else {
			if f < len(dbS)-1 {
				f++
			} else {
				break
			}
		}

	}
	return
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s RepoSearch) PrintSearch(mode int) {
	for i, res := range s {
		var toprint string
		if mode != -1 {
			if mode == 0 {
				toprint += fmt.Sprintf("%d ", len(s)-i-1)
			} else {
				toprint += fmt.Sprintf("%d ", i)
			}
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m",
			res.Repository, res.Name, res.Version)

		if len(res.Group) != 0 {
			toprint += fmt.Sprintf("(%s) ", res.Group)
		}

		if res.Installed == true {
			toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
		}

		toprint += "\n" + res.Description
		fmt.Println(toprint)
	}
}

// PFactory execute an action over a series of packages without reopening the handle everytime.
// Everybody told me it wouln't work. It does. It's just not pretty.
// When it worked: https://youtu.be/a4Z5BdEL0Ag?t=1m11s
func PFactory(action func(interface{})) func(name string, object interface{}, rel bool) {
	h, _ := conf.CreateHandle()
	localDb, _ := h.LocalDb()

	return func(name string, object interface{}, rel bool) {
		_, err := localDb.PkgByName(name)
		if err == nil {
			action(object)
		}

		if rel {
			h.Release()
		}
	}
}

// PackageSlices separates an input slice into aur and repo slices
func PackageSlices(toCheck []string) (aur []string, repo []string, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	dbList, err := h.SyncDbs()
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
			if _, err := dbList.PkgCachebyGroup(pkg); err == nil {
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
	h, _ := conf.CreateHandle()

	localDb, _ := h.LocalDb()
	dbList, _ := h.SyncDbs()

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}

	return func(toCheck []string, isBaseList bool, close bool) (repo []string, notFound []string) {
		if close {
			h.Release()
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
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return
	}
	dbList, err := h.SyncDbs()
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

	var cmd *exec.Cmd
	var args []string
	args = append(args, "pacman", "-S")
	args = append(args, pkgName...)
	if len(flags) != 0 {
		args = append(args, flags...)
	}

	cmd = exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	return nil
}

// CleanRemove sends a full removal command to pacman with the pkgName slice
func CleanRemove(pkgName []string) (err error) {
	if len(pkgName) == 0 {
		return nil
	}

	var cmd *exec.Cmd
	var args []string
	args = append(args, "pacman", "-Rnsc")
	args = append(args, pkgName...)
	args = append(args, "--noconfirm")

	cmd = exec.Command("sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Run()
	return nil
}

// ForeignPackages returns a map of foreign packages, with their version and date as values.
func ForeignPackages() (foreign map[string]*struct {
	Version string
	Date    int64
}, n int, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return
	}
	dbList, err := h.SyncDbs()
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

	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
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
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
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
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
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
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	localDb, err := h.LocalDb()
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
