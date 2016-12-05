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
		// Check if dep is installed
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
			aur = append(aur, pkg)
		}
	}

	return
}

// OutofRepo returns a list of packages not installed and not resolvable
// Accepts inputs like 'gtk2', 'java-environment=8', 'linux >= 4.20'
func OutofRepo(toCheck []string) (aur []string, repo []string, err error) {
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

toCheckLoop:
	for _, dep := range toCheck {
		field := strings.FieldsFunc(dep, f)

		for _, checkR := range repo {
			if field[0] == checkR {
				continue toCheckLoop
			}
		}

		for _, checkA := range aur {
			if field[0] == checkA {
				continue toCheckLoop
			}
		}

		// Check if dep is installed
		_, err = localDb.PkgByName(field[0])
		if err == nil {
			continue
		}

		found := false
	Loop:
		for _, db := range dbList.Slice() {
			// First, Check if they're provided by package name.
			_, err = db.PkgByName(field[0])
			if err == nil {
				found = true
				repo = append(repo, field[0])
				break Loop
			}

			for _, pkg := range db.PkgCache().Slice() {
				for _, p := range pkg.Provides().Slice() {
					if p.String() == dep {
						found = true
						repo = append(repo, pkg.Name())
						break Loop
					}
				}
			}
		}

		if !found {
			aur = append(aur, field[0])
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
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
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

	cmd = exec.Command("sudo", args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
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
func Statistics() (packages map[string]int64, info struct {
	Totaln    int
	Expln     int
	TotalSize int64
}, err error) {
	var pkgs [10]alpm.Package
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

	var k int
	for e, pkg := range localDb.PkgCache().Slice() {
		tS += pkg.ISize()
		k = -1
		nPkg++
		if pkg.Reason() == 0 {
			ePkg++
		}
		if e < 10 {
			pkgs[e] = pkg
			continue
		}

		for i, pkw := range pkgs {
			if k == -1 {
				if pkw.ISize() < pkg.ISize() {
					k = i
				}
			} else {
				if pkw.ISize() < pkgs[k].ISize() && pkw.ISize() < pkg.ISize() {
					k = i
				}
			}
		}

		if k != -1 {
			pkgs[k] = pkg
		}
	}

	packages = make(map[string]int64)
	for _, pkg := range pkgs {
		packages[pkg.Name()] = pkg.ISize()
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
