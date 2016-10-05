package aur

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jguer/go-alpm"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

var version = "undefined"

// TarBin describes the default installation point of tar command
// Probably will replace untar with code solution.
const TarBin string = "/usr/bin/tar"

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchMode is search without numbers.
const SearchMode int = -1

// Result describes an AUR package.
type Result struct {
	ID             int         `json:"ID"`
	Name           string      `json:"Name"`
	PackageBaseID  int         `json:"PackageBaseID"`
	PackageBase    string      `json:"PackageBase"`
	Version        string      `json:"Version"`
	Description    string      `json:"Description"`
	URL            string      `json:"URL"`
	NumVotes       int         `json:"NumVotes"`
	Popularity     float32     `json:"Popularity"`
	OutOfDate      interface{} `json:"OutOfDate"`
	Maintainer     string      `json:"Maintainer"`
	FirstSubmitted int         `json:"FirstSubmitted"`
	LastModified   int         `json:"LastModified"`
	URLPath        string      `json:"URLPath"`
	Depends        []string    `json:"Depends"`
	MakeDepends    []string    `json:"MakeDepends"`
	OptDepends     []string    `json:"OptDepends"`
	Conflicts      []string    `json:"Conflicts"`
	License        []string    `json:"License"`
	Keywords       []string    `json:"Keywords"`
	Installed      bool
}

// Query describes an AUR json Query
type Query struct {
	Resultcount int      `json:"resultcount"`
	Results     []Result `json:"results"`
	Type        string   `json:"type"`
	Version     int      `json:"version"`
}

// Editor gives the default system editor, uses vi in last case
var Editor = "vi"

func init() {
	if os.Getenv("EDITOR") != "" {
		Editor = os.Getenv("EDITOR")
	}
}

func (r Query) Len() int {
	return len(r.Results)
}

func (r Query) Less(i, j int) bool {
	return r.Results[i].NumVotes > r.Results[j].NumVotes
}

func (r Query) Swap(i, j int) {
	r.Results[i], r.Results[j] = r.Results[j], r.Results[i]
}

// PrintSearch handles printing search results in a given format
func (r *Query) PrintSearch(start int) {
	for i, res := range r.Results {
		switch {
		case start != SearchMode && res.Installed == true:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s \033[0m(%d) \x1B[32;40mInstalled\033[0m\n%s\n",
				start+i, "aur", res.Name, res.Version, res.NumVotes, res.Description)
		case start != SearchMode && res.Installed != true:
			fmt.Printf("%d \033[1m%s/\x1B[33m%s \x1B[36m%s \033[0m(%d)\n%s\n",
				start+i, "aur", res.Name, res.Version, res.NumVotes, res.Description)
		case start == SearchMode && res.Installed == true:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s \x1B[32;40mInstalled\033[0m\n%s\n",
				"aur", res.Name, res.Version, res.Description)
		case start == SearchMode && res.Installed != true:
			fmt.Printf("\033[1m%s/\x1B[33m%s \x1B[36m%s\033[0m\n%s\n",
				"aur", res.Name, res.Version, res.Description)
		}
	}
}

func downloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// getJSON handles JSON retrieval and decoding to struct
func getJSON(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

// Search returns an AUR search
func Search(pkg string, sortS bool) (r Query, err error) {
	err = getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkg, &r)
	if sortS {
		sort.Sort(r)
	}

	for i, res := range r.Results {
		r.Results[i].Installed, err = IspkgInstalled(res.Name)
	}
	return
}

// Info returns an AUR search with package details
func Info(pkg string) (r Query, err error) {
	err = getJSON("https://aur.archlinux.org/rpc/?v=5&type=info&arg[]="+pkg, &r)
	return
}

// Install sends system commands to make and install a package from pkgName
func Install(pkg string, baseDir string, conf *alpm.PacmanConfig, flags []string) (err error) {
	info, err := Info(pkg)
	if err != nil {
		return
	}

	if info.Resultcount == 0 {
		return errors.New("Package '" + pkg + "' does not exist")
	}

	info.Results[0].Install(baseDir, conf, flags)
	return err
}

// UpdatePackages handles AUR updates
func UpdatePackages(baseDir string, conf *alpm.PacmanConfig, flags []string) error {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return err
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return err
	}
	dbList, err := h.SyncDbs()
	if err != nil {
		return err
	}

	var foreign []alpm.Package
	var outdated []string

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
			foreign = append(foreign, pkg)
		}
	}

	// Find outdated packages
	for _, pkg := range foreign {
		info, err := Info(pkg.Name())
		if err != nil {
			return err
		}

		if info.Resultcount == 0 {
			continue
		}

		// Leaving this here for now, warn about downgrades later
		if int64(info.Results[0].LastModified) > pkg.InstallDate().Unix() {
			fmt.Printf("\033[1m\x1b[32m==>\x1b[33;1m %s: \033[0m%s \x1b[33;1m-> \033[0m%s\n",
				pkg.Name(), pkg.Version(), info.Results[0].Version)
			outdated = append(outdated, pkg.Name())
		}
	}

	//If there are no outdated packages, don't prompt
	if len(outdated) == 0 {
		return nil
	}

	// Install updated packages
	if NoConfirm(flags) == false {
		fmt.Println("\033[1m\x1b[32m==> Proceed with upgrade\033[0m\033[1m (Y/n)\033[0m")
		var response string
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "n & N") {
			return nil
		}
	}

	for _, pkg := range outdated {
		Install(pkg, baseDir, conf, flags)
	}

	return nil
}

// Install handles install from Result
func (a *Result) Install(baseDir string, conf *alpm.PacmanConfig, flags []string) (err error) {
	// No need to use filepath.separators because it won't run on inferior platforms
	err = os.MkdirAll(baseDir+"builds", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	tarLocation := baseDir + a.Name + ".tar.gz"
	defer os.Remove(baseDir + a.Name + ".tar.gz")

	err = downloadFile(tarLocation, BaseURL+a.URLPath)
	if err != nil {
		return
	}

	err = exec.Command(TarBin, "-xf", tarLocation, "-C", baseDir).Run()
	if err != nil {
		return
	}
	defer os.RemoveAll(baseDir + a.Name)
	_, err = a.Dependencies(conf)
	if err != nil {
		return
	}

	var response string
	var dir bytes.Buffer
	dir.WriteString(baseDir)
	dir.WriteString(a.Name)
	dir.WriteString("/")

	if NoConfirm(flags) == false {
		fmt.Println("\033[1m\x1b[32m==> Edit PKGBUILD?\033[0m\033[1m (y/N)\033[0m")
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "y & Y") {
			editcmd := exec.Command(Editor, dir.String()+"PKGBUILD")
			editcmd.Stdout = os.Stdout
			editcmd.Stderr = os.Stderr
			editcmd.Stdin = os.Stdin
			err = editcmd.Run()
		}
	}

	err = os.Chdir(dir.String())
	if err != nil {
		return
	}

	var makepkgcmd *exec.Cmd
	var args []string
	args = append(args, "-sri")
	args = append(args, flags...)
	makepkgcmd = exec.Command(MakepkgBin, args...)
	makepkgcmd.Stdout = os.Stdout
	makepkgcmd.Stderr = os.Stderr
	makepkgcmd.Stdin = os.Stdin
	err = makepkgcmd.Run()

	return
}

// Dependencies returns package dependencies splitting between AUR results and Repo Results not installed
func (a *Result) Dependencies(conf *alpm.PacmanConfig) (final []string, err error) {
	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}
	info, err := Info(a.Name)
	if err != nil {
		return
	}

	if len(info.Results) == 0 {
		return final, errors.New("Failed to get deps from RPC")
	}

	var found bool
	for _, deps := range info.Results[0].MakeDepends {
		fields := strings.FieldsFunc(deps, f)

		if found, err = IspkgInstalled(fields[0]); found {
			if err != nil {
				return
			}
			continue
		}

		if found, err = IspkgInRepo(fields[0], conf); !found {
			if err != nil {
				return
			}
			final = append(final, fields[0])
		}
	}

	for _, deps := range info.Results[0].Depends {
		fields := strings.FieldsFunc(deps, f)

		if found, err = IspkgInstalled(fields[0]); found {
			if err != nil {
				return
			}
			continue
		}

		if found, err = IspkgInRepo(fields[0], conf); !found {
			if err != nil {
				return
			}
			final = append(final, fields[0])
		}
	}
	return
}

// IspkgInstalled returns true if pkgName is installed
func IspkgInstalled(pkgName string) (bool, error) {
	h, err := alpm.Init("/", "/var/lib/pacman")
	defer h.Release()
	if err != nil {
		return false, err
	}

	localDb, err := h.LocalDb()
	if err != nil {
		return false, err
	}

	_, err = localDb.PkgByName(pkgName)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// IspkgInRepo returns true if pkgName is in a synced repo
func IspkgInRepo(pkgName string, conf *alpm.PacmanConfig) (bool, error) {
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return false, err
	}

	dbList, _ := h.SyncDbs()
	for _, db := range dbList.Slice() {
		_, err = db.PkgByName(pkgName)
		if err == nil {
			return true, nil
		}
	}
	return false, nil
}

// NoConfirm returns true if prompts should be ignored
func NoConfirm(flags []string) bool {
	noconf := false
	for _, flag := range flags {
		if strings.Contains(flag, "noconfirm") {
			noconf = true
			break
		}
	}

	return noconf
}
