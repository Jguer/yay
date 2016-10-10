package main

import (
	"bufio"
	"fmt"
	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/aur"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func searchAndInstall(pkgName string, conf *alpm.PacmanConfig, flags []string) (err error) {
	var num int
	var numberString string
	var args []string

	a, err := aur.Search(pkgName, true)
	r, err := SearchPackages(pkgName, conf)
	if err != nil {
		return
	}

	if len(r.Results) == 0 && a.Resultcount == 0 {
		return fmt.Errorf("No Packages match search.")
	}
	r.PrintSearch(0)
	a.PrintSearch(len(r.Results))

	args = append(args, "pacman", "-S")

	fmt.Printf("\x1B[32m%s\033[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
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

// updateAndInstall handles updating the cache and installing updates
func updateAndInstall(conf *alpm.PacmanConfig, flags []string) error {
	errp := UpdatePackages(flags)
	erra := aur.UpdatePackages(BuildDir, conf, flags)

	if errp != nil {
		return errp
	}

	return erra
}

func searchMode(pkg string, conf *alpm.PacmanConfig) (err error) {
	a, err := aur.Search(pkg, true)
	if err != nil {
		return err
	}

	SearchRepos(pkg, conf, SearchMode)
	a.PrintSearch(SearchMode)

	return nil
}

func stats(conf *alpm.PacmanConfig) error {
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

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", nPkg)
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", ePkg)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", Size(tS))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	for _, pkg := range pkgs {
		fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkg.Name(), Size(pkg.ISize()))
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
func Size(s int64) string {

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
