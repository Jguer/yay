package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/Jguer/go-alpm"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func searchAurPackages(pkg string, index int) (search AurSearch, err error) {
	err = getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkg, &search)
	if index != SearchMode {
		sort.Sort(search)
	}
	return
}

// SearchPackages handles repo searches
func SearchPackages(pkg string) (search RepoSearch, err error) {
	h, er := alpm.Init("/", "/var/lib/pacman")
	if er != nil {
		fmt.Println(er)
		return
	}
	defer h.Release()

	fmt.Println("before dblist")
	dbList, _ := h.SyncDbs()
	fmt.Println("after dblist")
	// db, _ := h.RegisterSyncDb("core", 0)
	// h.RegisterSyncDb("community", 0)
	// h.RegisterSyncDb("extra", 0)

	_, err = h.SyncDbByName("core")
	fmt.Println(err)
	fmt.Printf("%+v\n", dbList)

    db, _ := h.LocalDb()
    for _, pkg := range db.PkgCache().Slice() {
        fmt.Printf("%s %s\n  %s\n",
        pkg.Name(), pkg.Version(), pkg.Description())
    }

	for _, db := range dbList.Slice() {
		fmt.Printf("%+v\n", db)
		db, _ := h.LocalDb()
		for _, pkg := range db.PkgCache().Slice() {
			fmt.Printf("%s %s\n  %s\n",
				pkg.Name(), pkg.Version(), pkg.Description())
		}
	}
	return
}

// SearchPackagesa handles repo searches
func SearchPackagesa(pkg string) (search RepoSearch, err error) {
	cmdOutput, err := exec.Command(PacmanBin, "-Ss", pkg).Output()
	outputSlice := strings.Split(string(cmdOutput), "\n")
	if outputSlice[0] == "" {
		return search, nil
	}

	i := true
	var tempStr string
	var rRes *RepoResult
	for _, pkgStr := range outputSlice {
		if i {
			rRes = new(RepoResult)
			fmt.Sscanf(pkgStr, "%s %s\n", &tempStr, &rRes.Version)
			repoNameSlc := strings.Split(tempStr, "/")
			rRes.Repository = repoNameSlc[0]
			rRes.Name = repoNameSlc[1]
			i = false
		} else {
			rRes.Description = pkgStr
			search.Resultcount++
			search.Results = append(search.Results, *rRes)
			i = true
		}
	}
	return
}

func infoAurPackage(pkg string) (info AurSearch, err error) {
	err = getJSON("https://aur.archlinux.org/rpc/?v=5&type=info&arg[]="+pkg, &info)
	return
}

func (r AurSearch) printSearch(index int) (err error) {
	for i, result := range r.Results {
		if index != SearchMode {
			fmt.Printf("%d \033[1maur/\x1B[33m%s \x1B[36m%s\033[0m (%d)\n    %s\n",
				i+index, result.Name, result.Version, result.NumVotes, result.Description)
		} else {
			fmt.Printf("\033[1maur/\x1B[33m%s \x1B[36m%s\033[0m (%d)\n    %s\n",
				result.Name, result.Version, result.NumVotes, result.Description)
		}
	}

	return
}

func (s RepoSearch) printSearch(index int) (err error) {
	for i, result := range s.Results {
		if index != SearchMode {
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				i, result.Repository, result.Name, result.Version, result.Description)
		} else {
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				result.Repository, result.Name, result.Version, result.Description)
		}
	}

	return nil
}

// To implement
func (a AurResult) getDepsfromFile(pkgbuildLoc string) (err error) {
	var depend string
	file, err := os.Open(pkgbuildLoc)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "optdepends=(") {
			continue
		}
		if strings.Contains(scanner.Text(), "depends=(") {
			depend = scanner.Text()
			fields := strings.Fields(depend)

			for _, i := range fields {
				fmt.Println(i)
			}
			break
		}
	}

	return nil
}

func (a AurResult) getDepsFromRPC() (final []string, err error) {
	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}
	info, err := infoAurPackage(a.Name)
	if err != nil {
		return
	}

	if len(info.Results) == 0 {
		return final, errors.New("Failed to get deps from RPC")
	}

	for _, deps := range info.Results[0].MakeDepends {
		fields := strings.FieldsFunc(deps, f)
		if !isInRepo(fields[0]) {
			final = append(final, fields[0])
		}
	}

	for _, deps := range info.Results[0].Depends {
		fields := strings.FieldsFunc(deps, f)
		if !isInRepo(fields[0]) {
			final = append(final, fields[0])
		}
	}

	return
}

func (a AurResult) getAURDependencies() (err error) {
	pkglist, err := a.getDepsFromRPC()
	fmt.Printf("%+v\n", pkglist)

	for _, i := range pkglist {
		err = installAURPackage(i, "--asdeps")
		if err != nil {
			for _, e := range pkglist {
				removePackage(e, "sdc")
			}
			return
		}
	}
	return nil
}

func getInstalledPackage(pkg string) (err error) {
	cmd := exec.Command(PacmanBin, "-Qi", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return
}
