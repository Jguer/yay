package pacman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jguer/go-alpm"
)

// RepoSearch describes a Repository search.
type RepoSearch struct {
	Results []Result
}

// Result describes a pkg.
type Result struct {
	Name        string
	Repository  string
	Version     string
	Description string
	Installed   bool
}

// SearchMode is search without numbers.
const SearchMode int = -1

// PacmanConf describes the default pacman config file
const PacmanConf string = "/etc/pacman.conf"

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

// SearchRepos searches and prints packages in repo
func SearchRepos(pkgName string, mode int) (err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	dbList, _ := h.SyncDbs()
	localdb, _ := h.LocalDb()

	var installed bool
	var i int
	for _, db := range dbList.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			if strings.Contains(pkg.Name(), pkgName) {
				if r, _ := localdb.PkgByName(pkg.Name()); r != nil {
					installed = true
				} else {
					installed = false
				}

				switch {
				case mode != SearchMode && !installed:
					fmt.Printf("%d \x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[32;40mInstalled\x1b[0m\n%s\n",
						i, db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode != SearchMode && !installed:
					fmt.Printf("%d \x1b[1m%s/\x1b[33m%s \x1b[36m%s\x1b[0m\n%s\n",
						i, db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode == SearchMode && !installed:
					fmt.Printf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[32;40mInstalled\x1b[0m\n%s\n",
						db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				case mode == SearchMode && !installed:
					fmt.Printf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s\x1b[0m\n%s\n",
						db.Name(), pkg.Name(), pkg.Version(), pkg.Description())
				}
				i++
			}
		}
	}
	return
}

// SearchPackages handles repo searches. Creates a RepoSearch struct.
func SearchPackages(pkgName string) (s RepoSearch, err error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	dbList, _ := h.SyncDbs()
	localdb, _ := h.LocalDb()

	var installed bool
	for _, db := range dbList.Slice() {
		for _, pkg := range db.PkgCache().Slice() {
			if strings.Contains(pkg.Name(), pkgName) {
				if r, _ := localdb.PkgByName(pkg.Name()); r != nil {
					installed = true
				} else {
					installed = false
				}

				s.Results = append(s.Results, Result{
					Name:        pkg.Name(),
					Description: pkg.Description(),
					Version:     pkg.Version(),
					Repository:  db.Name(),
					Installed:   installed,
				})
			}
		}
	}
	return
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s *RepoSearch) PrintSearch(mode int) {
	for i, pkg := range s.Results {
		switch {
		case mode != SearchMode && pkg.Installed:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
				i, pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode != SearchMode && !pkg.Installed:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				i, pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode == SearchMode && pkg.Installed:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
				pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		case mode == SearchMode && !pkg.Installed:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				pkg.Repository, pkg.Name, pkg.Version, pkg.Description)
		}
	}
}

// PassToPacman outsorces execution to pacman binary without modifications.
func PassToPacman(op string, pkgs []string, flags []string) error {
	var cmd *exec.Cmd
	var args []string

	args = append(args, op)
	if len(pkgs) != 0 {
		args = append(args, pkgs...)
	}

	if len(flags) != 0 {
		args = append(args, flags...)
	}

	if strings.Contains(op, "-Q") {
		cmd = exec.Command("pacman", args...)
	} else {
		args = append([]string{"pacman"}, args...)
		cmd = exec.Command("sudo", args...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err

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

	// First, Check if they're provided by package name.
	for _, dep := range toCheck {
		field := strings.FieldsFunc(dep, f)

		// Check if dep is installed
		_, err = localDb.PkgByName(field[0])
		if err == nil {
			continue
		}

		found := false
	Loop:
		for _, db := range dbList.Slice() {
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

	return
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
