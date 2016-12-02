package aur

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/jguer/yay/pacman"
)

// TarBin describes the default installation point of tar command.
const TarBin string = "/usr/bin/tar"

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchMode is search without numbers.
const SearchMode int = -1

// SortMode determines top down package or down top package display
var SortMode = DownTop

// Describes Sorting method for numberdisplay
const (
	DownTop = iota
	TopDown
)

// Result describes an AUR package.
type Result struct {
	ID             int     `json:"ID"`
	Name           string  `json:"Name"`
	PackageBaseID  int     `json:"PackageBaseID"`
	PackageBase    string  `json:"PackageBase"`
	Version        string  `json:"Version"`
	Description    string  `json:"Description"`
	URL            string  `json:"URL"`
	NumVotes       int     `json:"NumVotes"`
	Popularity     float32 `json:"Popularity"`
	OutOfDate      int     `json:"OutOfDate"`
	Maintainer     string  `json:"Maintainer"`
	FirstSubmitted int     `json:"FirstSubmitted"`
	LastModified   int64   `json:"LastModified"`
	URLPath        string  `json:"URLPath"`
	Installed      bool
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	OptDepends     []string `json:"OptDepends"`
	Conflicts      []string `json:"Conflicts"`
	Provides       []string `json:"Provides"`
	License        []string `json:"License"`
	Keywords       []string `json:"Keywords"`
}

// Query is a collection of Results
type Query []Result

func (q Query) Len() int {
	return len(q)
}

func (q Query) Less(i, j int) bool {
	if SortMode == DownTop {
		return q[i].NumVotes < q[j].NumVotes
	}
	return q[i].NumVotes > q[j].NumVotes
}

func (q Query) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// PrintSearch handles printing search results in a given format
func (q Query) PrintSearch(start int) {
	for i, res := range q {
		var toprint string
		if start != SearchMode {
			if SortMode == DownTop {
				toprint += fmt.Sprintf("%d ", len(q)+start-i-1)
			} else {
				toprint += fmt.Sprintf("%d ", start+i)
			}
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m(%d) ", "aur", res.Name, res.Version, res.NumVotes)
		if res.Installed == true {
			toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
		}
		toprint += "\n" + res.Description
		fmt.Println(toprint)
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
	setter := pacman.PFactory(pFSetTrue)

	for i, res := range r.Results {
		if i == len(r.Results)-1 {
			setter(res.Name, &r.Results[i], true)
			continue
		}
		setter(res.Name, &r.Results[i], false)
	}
	return r.Results, r.ResultCount, err
}

// This is very dirty but it works so good.
func pFSetTrue(res interface{}) {
	f, ok := res.(*Result)
	if !ok {
		fmt.Println("Unable to convert back to Result")
		return
	}
	f.Installed = true

	return
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
func Install(pkg string, baseDir string, flags []string) (err error) {
	q, n, err := Info(pkg)
	if err != nil {
		return
	}

	if n == 0 {
		return fmt.Errorf("Package %s does not exist", pkg)
	}

	q[0].Install(baseDir, flags)
	return err
}

// Upgrade tries to update every foreign package installed in the system
func Upgrade(baseDir string, flags []string) error {
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
	if !NoConfirm(flags) {
		fmt.Println("\x1b[1m\x1b[32m==> Proceed with upgrade\x1b[0m\x1b[1m (Y/n)\x1b[0m")
		var response string
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "n & N") {
			return nil
		}
	}

	for _, pkg := range outdated {
		pkg.Install(baseDir, flags)
	}

	return nil
}

// Install handles install from Result
func (a *Result) Install(baseDir string, flags []string) (err error) {
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

	if !NoConfirm(flags) {
		fmt.Println("\x1b[1m\x1b[32m==> Edit PKGBUILD?\x1b[0m\x1b[1m (y/N)\x1b[0m")
		fmt.Scanln(&response)
		if strings.ContainsAny(response, "y & Y") {
			editcmd := exec.Command(Editor, dir.String()+"PKGBUILD")
			editcmd.Stdout = os.Stdout
			editcmd.Stderr = os.Stderr
			editcmd.Stdin = os.Stdin
			editcmd.Run()
		}
	}
	aurDeps, repoDeps, err := a.Dependencies()
	if err != nil {
		return
	}

	aurQ, n, err := MultiInfo(aurDeps)
	if n != len(aurDeps) {
		fmt.Printf("Unable to find a dependency on AUR")
	}

	// Handle AUR dependencies first
	for _, dep := range aurQ {
		dep.Install(baseDir, []string{"--asdeps"})
	}

	// Repo dependencies
	if len(repoDeps) != 0 {
		pacman.Install(repoDeps, []string{"--asdeps", "--needed"})
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

// Dependencies returns package dependencies not installed belonging to AUR
func (a *Result) Dependencies() (aur []string, repo []string, err error) {
	var q Query
	if len(a.Depends) == 0 && len(a.MakeDepends) == 0 {
		var n int
		q, n, err = Info(a.Name)
		if n == 0 || err != nil {
			err = fmt.Errorf("Unable to search dependencies, %s", err)
			return
		}
	} else {
		q = append(q, *a)
	}

	aur, repo, err = pacman.OutofRepo(append(q[0].MakeDepends, q[0].Depends...))
	return
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
