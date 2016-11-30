package aur

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	alpm "github.com/demizer/go-alpm"
	"github.com/jguer/yay/pacman"
)

var version = "undefined"

// TarBin describes the default installation point of tar command.
const TarBin string = "/usr/bin/tar"

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchMode is search without numbers.
const SearchMode int = -1

// Result describes an AUR package.
type Result struct {
	ID             int      `json:"ID"`
	Name           string   `json:"Name"`
	PackageBaseID  int      `json:"PackageBaseID"`
	PackageBase    string   `json:"PackageBase"`
	Version        string   `json:"Version"`
	Description    string   `json:"Description"`
	URL            string   `json:"URL"`
	NumVotes       int      `json:"NumVotes"`
	Popularity     float32  `json:"Popularity"`
	OutOfDate      int      `json:"OutOfDate"`
	Maintainer     string   `json:"Maintainer"`
	FirstSubmitted int      `json:"FirstSubmitted"`
	LastModified   int64    `json:"LastModified"`
	URLPath        string   `json:"URLPath"`
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	OptDepends     []string `json:"OptDepends"`
	Conflicts      []string `json:"Conflicts"`
	License        []string `json:"License"`
	Keywords       []string `json:"Keywords"`
	Installed      bool
}

// Query is a collection of Results
type Query []Result

func (q Query) Len() int {
	return len(q)
}

func (q Query) Less(i, j int) bool {
	return q[i].NumVotes < q[j].NumVotes
}

func (q Query) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// PrintSearch handles printing search results in a given format
func (q Query) PrintSearch(start int) {
	for i, res := range q {
		switch {
		case start != SearchMode && res.Installed == true:
			fmt.Printf("%d \x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m(%d) \x1b[32;40mInstalled\x1b[0m\n%s\n",
				start+i, "aur", res.Name, res.Version, res.NumVotes, res.Description)
		case start != SearchMode && res.Installed != true:
			fmt.Printf("%d \x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m(%d)\n%s\n",
				start+i, "aur", res.Name, res.Version, res.NumVotes, res.Description)
		case start == SearchMode && res.Installed == true:
			fmt.Printf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[32;40mInstalled\x1b[0m\n%s\n",
				"aur", res.Name, res.Version, res.Description)
		case start == SearchMode && res.Installed != true:
			fmt.Printf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s\x1b[0m\n%s\n",
				"aur", res.Name, res.Version, res.Description)
		}
	}
}

// Search returns an AUR search
func Search(pkg string, sortS bool) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}
	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkg, &r)

	if sortS {
		sort.Sort(r.Results)
	}

	// for _, res := range r.Results {
	// 	res.Installed, err = IspkgInstalled(res.Name)
	// }
	return r.Results, r.ResultCount, err
}

// Info returns an AUR search with package details
func Info(pkg string) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}

	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=info&arg[]="+pkg, &r)

	return r.Results, r.ResultCount, err
}

// MultiInfo takes a slice of strings and returns a slice with the info of each package
func MultiInfo(pkgS []string) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}

	var pkg string
	for _, pkgn := range pkgS {
		pkg += "&arg[]=" + pkgn
	}

	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=info"+pkg, &r)

	return r.Results, r.ResultCount, err
}

// Install sends system commands to make and install a package from pkgName
func Install(pkg string, baseDir string, conf *alpm.PacmanConfig, flags []string) (err error) {
	q, n, err := Info(pkg)
	if err != nil {
		return
	}

	if n == 0 {
		return fmt.Errorf("Package %s does not exist", pkg)
	}

	q[0].Install(baseDir, conf, flags)
	return err
}

// Upgrade tries to update every foreign package installed in the system
func Upgrade(baseDir string, conf *alpm.PacmanConfig, flags []string) error {
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

	q, _, err := MultiInfo(keys)
	if err != nil {
		return err
	}

	outdated := q[:0]
	for _, res := range q {
		if _, ok := foreign[res.Name]; ok {
			// Leaving this here for now, warn about downgrades later
			if res.LastModified > foreign[res.Name].Date {
				// o[i] = o[len(o)-1]
				// o[len(o)-1] = Result{} // Trying to help the GC, not sure if necessary. Time will tell
				// o = o[:len(o)-1]
				fmt.Printf("\x1b[1m\x1b[32m==>\x1b[33;1m %s: \x1b[0m%s \x1b[33;1m-> \x1b[0m%s\n",
					res.Name, res.Version, foreign[res.Name].Version)
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
	if NoConfirm(flags) == false {
		fmt.Println("\x1b[1m\x1b[32m==> Proceed with upgrade\x1b[0m\x1b[1m (Y/n)\x1b[0m")
		var response string
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "n & N") {
			return nil
		}
	}

	for _, pkg := range outdated {
		pkg.Install(baseDir, conf, flags)
	}

	return nil
}


// Install handles install from Result
func (a *Result) Install(baseDir string, conf *alpm.PacmanConfig, flags []string) (err error) {
	fmt.Printf("\x1b[1m\x1b[32m==> Installing\x1b[33m %s\x1b[0m\n", a.Name)

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

	var response string
	var dir bytes.Buffer
	dir.WriteString(baseDir)
	dir.WriteString(a.Name)
	dir.WriteString("/")

	if NoConfirm(flags) == false {
		fmt.Println("\x1b[1m\x1b[32m==> Edit PKGBUILD?\x1b[0m\x1b[1m (y/N)\x1b[0m")
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "y & Y") {
			editcmd := exec.Command(Editor, dir.String()+"PKGBUILD")
			editcmd.Stdout = os.Stdout
			editcmd.Stderr = os.Stderr
			editcmd.Stdin = os.Stdin
			err = editcmd.Run()
		}
	}
	depS, err := a.Dependencies(conf)
	if err != nil {
		return
	}

	for _, dep := range depS {
		q, n, errD := Info(dep)
		if errD != nil {
			return errD
		}

		if n != 0 {
			q[0].Install(baseDir, conf, []string{"--asdeps"})
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
	h, err := conf.CreateHandle()
	defer h.Release()
	if err != nil {
		return
	}

	dbList, err := h.SyncDbs()
	localDb, err := h.LocalDb()
	if err != nil {
		return
	}

	f := func(c rune) bool {
		return c == '>' || c == '<' || c == '=' || c == ' '
	}
	q, n, err := Info(a.Name)
	if err != nil {
		return
	}

	if n == 0 {
		return final, fmt.Errorf("Failed to get deps from RPC")
	}

	deps := append(q[0].MakeDepends, q[0].Depends...)
	for _, dep := range deps {
		fields := strings.FieldsFunc(dep, f)
		// If package is installed let it go.
		_, err = localDb.PkgByName(fields[0])
		if err == nil {
			continue
		}

		// If package is in repo let it be installed by makepkg.
		found := false
		for _, db := range dbList.Slice() {
			_, err = db.PkgByName(fields[0])
			if err == nil {
				found = true
			}
		}

		if found {
			continue
		}

		_, nd, err := Info(fields[0])
		if err != nil {
			return final, err
		}

		if nd == 0 {
			return final, fmt.Errorf("Unable to find dependency in repos and AUR.")
		}

		final = append(final, fields[0])
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
