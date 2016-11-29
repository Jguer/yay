package yay

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jguer/go-alpm"
	"github.com/jguer/yay/aur"
)

// BuildDir is the root for package building
const BuildDir string = "/tmp/yaytmp/"

// SearchMode is search without numbers.
const SearchMode int = -1

// NumberMenu presents a CLI for selecting packages to install.
func NumberMenu(pkgName string, flags []string) (err error) {
	var num int
	var numberString string
	var args []string

	a, err := aur.Search(pkgName, true)
	r, err := SearchPackages(pkgName, conf)
	if err != nil {
		return
	}

	if len(r.Results) == 0 && a.Resultcount == 0 {
		return fmt.Errorf("no Packages match search")
	}
	r.PrintSearch(0)
	a.PrintSearch(len(r.Results))

	args = append(args, "pacman", "-S")

	fmt.Printf("\x1b[32m%s\x1b[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberString, err = reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	var aurInstall []aur.Result
	result := strings.Fields(numberString)
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Install package
		if num > len(r.Results)-1 {
			aurInstall = append(aurInstall, a.Results[num-len(r.Results)])
		} else {
			args = append(args, r.Results[num].Name)
		}
	}

	args = append(args, flags...)

	if len(args) > 2 {
		var cmd *exec.Cmd
		cmd = exec.Command("sudo", args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	for _, aurpkg := range aurInstall {
		err = aurpkg.Install(BuildDir, conf, flags)
		if err != nil {
			// Do not abandon program, we might still be able to install the rest
			fmt.Println(err)
		}
	}
	return
}

// Install handles package installs
func Install(pkgs []string, flags []string) error {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return err
	}

	dbList, err := h.SyncDbs()
	if err != nil {
		return err
	}

	var foreign []string
	var args []string
	repocnt := 0
	args = append(args, "pacman")
	args = append(args, "-S")

	for _, pkg := range pkgs {
		found := false
		for _, db := range dbList.Slice() {
			_, err = db.PkgByName(pkg)
			if err == nil {
				found = true
				args = append(args, pkg)
				repocnt++
				break
			}
		}

		if !found {
			foreign = append(foreign, pkg)
		}
	}

	args = append(args, flags...)

	if repocnt != 0 {
		var cmd *exec.Cmd
		cmd = exec.Command("sudo", args...)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	}

	for _, aurpkg := range foreign {
		err = aur.Install(aurpkg, BuildDir, conf, flags)
	}

	return nil
}

// Upgrade handles updating the cache and installing updates.
func Upgrade(flags []string) error {
	errp := UpdatePackages(flags)
	erra := aur.UpdatePackages(BuildDir, conf, flags)

	if errp != nil {
		return errp
	}

	return erra
}

// Search presents a query to the local repos and to the AUR.
func Search(pkg string) (err error) {
	a, err := aur.Search(pkg, true)
	if err != nil {
		return err
	}

	SearchRepos(pkg, conf, SearchMode)
	a.PrintSearch(SearchMode)

	return nil
}

// LocalStatistics returns installed packages statistics.
func LocalStatistics() error {
	var tS int64 // TotalSize
	var nPkg int
	var ePkg int
	var pkgs [10]alpm.Package
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return err
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return err
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

	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", nPkg)
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", ePkg)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", size(tS))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	for _, pkg := range pkgs {
		fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), size(pkg.ISize()))
	}
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")

	return nil
}

// Function by pyk https://github.com/pyk/byten
func index(s int64) float64 {
	x := math.Log(float64(s)) / math.Log(1024)
	return math.Floor(x)
}

// Function by pyk https://github.com/pyk/byten
func countSize(s int64, i float64) float64 {
	return float64(s) / math.Pow(1024, math.Floor(i))
}

// Size return a formated string from file size
// Function by pyk https://github.com/pyk/byten
func size(s int64) string {

	symbols := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	i := index(s)
	if s < 10 {
		return fmt.Sprintf("%dB", s)
	}
	size := countSize(s, i)
	format := "%.0f"
	if size < 10 {
		format = "%.1f"
	}

	return fmt.Sprintf(format+"%s", size, symbols[int(i)])
}
