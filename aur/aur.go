package aur

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	alpm "github.com/jguer/go-alpm"
	vcs "github.com/jguer/yay/aur/vcs"
	"github.com/jguer/yay/config"
	"github.com/jguer/yay/pacman"
	rpc "github.com/mikkeloscar/aur"
)

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

var specialDBsauce bool = false

// NarrowSearch searches AUR and narrows based on subarguments
func NarrowSearch(pkgS []string, sortS bool) (Query, error) {
	if len(pkgS) == 0 {
		return nil, nil
	}

	r, err := rpc.Search(pkgS[0])
	if err != nil {
		return nil, err
	}

	if len(pkgS) == 1 {
		if sortS {
			sort.Sort(Query(r))
		}
		return r, err
	}

	var aq Query
	var n int

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
		fmt.Printf("Some package from list\n%+v\ndoes not exist", pkgName)
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

// CreateDevelDB forces yay to create a DB of the existing development packages
func CreateDevelDB() error {
	foreign, err := pacman.ForeignPackages()
	if err != nil {
		return err
	}

	keys := make([]string, len(foreign))
	i := 0
	for k := range foreign {
		keys[i] = k
		i++
	}

	config.YayConf.NoConfirm = true
	specialDBsauce = true
	err = Install(keys, nil)
	return err
}

func develUpgrade(foreign map[string]alpm.Package, flags []string) error {
	fmt.Println(" Checking development packages...")
	develUpdates := vcs.CheckUpdates(foreign)
	if len(develUpdates) != 0 {
		for _, q := range develUpdates {
			fmt.Printf("\x1b[1m\x1b[32m==>\x1b[33;1m %s\x1b[0m\n", q)
		}
		// Install updated packages
		if !config.ContinueTask("Proceed with upgrade?", "nN") {
			return nil
		}

		err := Install(develUpdates, flags)
		if err != nil {
			fmt.Println(err)
		}
	}

	return nil
}

type upgrade struct {
	Name          string
	LocalVersion  string
	RemoteVersion string
}

func UpgradeList(flags []string) (toUpgrade []upgrade, err error) {
	foreign, foreignNames, err := pacman.ForeignPackageList()
	if err != nil {
		return
	}

	var qtemp Query
	var j int
	var routines int
	var routineDone int

	packageC := make(chan upgrade)
	done := make(chan bool)

	for i := len(foreign); i != 0; i = j {
		j = i - config.YayConf.RequestSplitN
		if j < 0 {
			j = 0
		}

		qtemp, err = rpc.Info(foreignNames[j:i])
		if err != nil {
			return
		}

		routines++
		go func(qtemp Query, local []alpm.Package) {
			// For each item in query: Search equivalent in foreign.
			// We assume they're ordered and are returned ordered
			// and will only be missing if they don't exist in AUR.
			max := len(qtemp) - 1
			var missing, x int

			fmt.Print("\n")
			for i, _ := range local {
				x = i - missing
				if x > max {
					break
				} else if qtemp[x].Name == local[i].Name() {
					if (config.YayConf.TimeUpdate && (int64(qtemp[x].LastModified) > local[i].BuildDate().Unix())) ||
						(alpm.VerCmp(local[i].Version(), qtemp[x].Version) < 0) {
						packageC <- upgrade{qtemp[x].Name, local[i].Version(), qtemp[x].Version}
						continue
					}
				} else {
					missing++
				}
			}
			done <- true
		}(qtemp, foreign[j:i])
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

// Upgrade tries to update every foreign package installed in the system
func Upgrade(flags []string) error {
	fmt.Println("\x1b[1;36;1m::\x1b[0m\x1b[1m Starting AUR upgrade...\x1b[0m")

	foreign, err := pacman.ForeignPackages()
	if err != nil {
		return err
	}
	keys := make([]string, len(foreign))
	i := 0
	for k := range foreign {
		keys[i] = k
		i++
	}

	if config.YayConf.Devel {
		err := develUpgrade(foreign, flags)
		if err != nil {
			fmt.Println(err)
		}
	}

	var q Query
	var j int
	for i = len(keys); i != 0; i = j {
		j = i - config.YayConf.RequestSplitN
		if j < 0 {
			j = 0
		}
		qtemp, err := rpc.Info(keys[j:i])
		q = append(q, qtemp...)
		if err != nil {
			return err
		}
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	outdated := q[:0]
	for i, res := range q {
		fmt.Printf("\r Checking %d/%d packages...", i+1, len(q))

		if _, ok := foreign[res.Name]; ok {
			// Leaving this here for now, warn about downgrades later
			if (config.YayConf.TimeUpdate && (int64(res.LastModified) > foreign[res.Name].BuildDate().Unix())) ||
				alpm.VerCmp(foreign[res.Name].Version(), res.Version) < 0 {
				buffer.WriteString(fmt.Sprintf("\x1b[1m\x1b[32m==>\x1b[33;1m %s: \x1b[0m%s \x1b[33;1m-> \x1b[0m%s\n",
					res.Name, foreign[res.Name].Version(), res.Version))
				outdated = append(outdated, res)
			}
		}
	}
	fmt.Println(buffer.String())

	//If there are no outdated packages, don't prompt
	if len(outdated) == 0 {
		fmt.Println("there is nothing to do")
		return nil
	}

	// Install updated packages
	if !config.ContinueTask("Proceed with upgrade?", "nN") {
		return nil
	}

	var finalmdeps []string
	for _, pkgi := range outdated {
		mdeps, err := PkgInstall(&pkgi, flags)
		finalmdeps = append(finalmdeps, mdeps...)
		if err != nil {
			fmt.Println(err)
		}
	}

	err = pacman.CleanRemove(finalmdeps)
	if err != nil {
		fmt.Println(err)
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
	config.DownloadAndUnpack(BaseURL+aq[0].URLPath, dir, false)
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
		if config.YayConf.Shell == "fish" {
			fmt.Print("\tAUR\n")
			out.WriteString("\tAUR\n")
		} else {
			fmt.Print("\n")
			out.WriteString("\n")
		}
	}

	return nil
}
