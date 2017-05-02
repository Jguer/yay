package aur

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
	rpc "github.com/mikkeloscar/aur"
)

// NarrowSearch searches AUR and narrows based on subarguments
func NarrowSearch(pkgS []string, sortS bool) (Query, error) {
	if len(pkgS) == 0 {
		return nil, nil
	}

	r, err := rpc.Search(pkgS[0])

	if len(pkgS) == 1 {
		if sortS {
			sort.Sort(Query(r))
		}
		return r, err
	}

	var aq Query
	var n int = 0

	for _, res := range r {
		match := true
		for _, pkgN := range pkgS[1:] {
			if !(strings.Contains(res.Name, pkgN) || strings.Contains(strings.ToLower(res.Description), pkgN)) {
				match = false
				break
			}
		}

		if match {
			n++
			aq = append(aq, res)
		}
	}

	if sortS {
		sort.Sort(aq)
	}

	return aq, err
}

// Install sends system commands to make and install a package from pkgName
func Install(pkgName []string, flags []string) (err error) {
	q, err := rpc.Info(pkgName)
	if err != nil {
		return
	}

	if len(q) != len(pkgName) {
		return fmt.Errorf("Some package from list\n%+v\ndoes not exist", pkgName)
	}

	var finalrm []string
	for _, i := range q {
		mrm, err := PkgInstall(&i, flags)
		if err != nil {
			fmt.Println("Error installing", i.Name, ":", err)
		}
		finalrm = append(finalrm, mrm...)
	}

	if len(finalrm) != 0 {
		err = RemoveMakeDeps(finalrm)
	}

	return err
}

// Upgrade tries to update every foreign package installed in the system
func Upgrade(flags []string) error {
	fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Starting AUR upgrade...\x1b[0m")

	foreign, n, err := pacman.ForeignPackages()
	if err != nil || n == 0 {
		return err
	}

	keys := make([]string, len(foreign))
	i := 0
	for k := range foreign {
		keys[i] = k
		i++
	}

	q, err := rpc.Info(keys)
	if err != nil {
		return err
	}

	outdated := q[:0]
	for _, res := range q {
		if _, ok := foreign[res.Name]; ok {
			// Leaving this here for now, warn about downgrades later
			if res.LastModified > int(foreign[res.Name].Date) {
				fmt.Printf("\x1b[1m\x1b[32m==>\x1b[33;1m %s: \x1b[0m%s \x1b[33;1m-> \x1b[0m%s\n",
					res.Name, foreign[res.Name].Version, res.Version)
				outdated = append(outdated, res)
			}
		}
	}

	//If there are no outdated packages, don't prompt
	if len(outdated) == 0 {
		fmt.Println(" there is nothing to do")
		return nil
	}

	// Install updated packages
	if !util.ContinueTask("Proceed with upgrade?", "nN") {
		return nil
	}

	for _, pkgi := range outdated {
		PkgInstall(&pkgi, flags)
	}

	return nil
}

// GetPkgbuild downloads pkgbuild from the AUR.
func GetPkgbuild(pkgN string, dir string) (err error) {
	aq, err := rpc.Info([]string{pkgN})
	if err != nil {
		return err
	}

	if len(aq) == 0 {
		return fmt.Errorf("no results")
	}

	fmt.Printf("\x1b[1;32m==>\x1b[1;33m %s \x1b[1;32mfound in AUR.\x1b[0m\n", pkgN)
	util.DownloadAndUnpack(BaseURL+aq[0].URLPath, dir, false)
	return
}

//CreateAURList creates a new completion file
func CreateAURList(out *os.File) (err error) {
	resp, err := http.Get("https://aur.archlinux.org/packages.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()
	for scanner.Scan() {
		fmt.Print(scanner.Text())
		out.WriteString(scanner.Text())
		if util.Shell == "fish" {
			fmt.Print("\tAUR\n")
			out.WriteString("\tAUR\n")
		} else {
			fmt.Print("\n")
			out.WriteString("\n")
		}
	}

	return nil
}
