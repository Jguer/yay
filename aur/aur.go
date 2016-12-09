package aur

import (
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

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode determines top down package or down top package display
var SortMode = DownTop

// BaseDir is the default building directory for yay
var BaseDir = "/tmp/yaytmp/"

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
		if res.Maintainer == "" {
			toprint += fmt.Sprintf("\x1b[31;40m(Orphaned)\x1b[0m ")
		}

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
func Install(pkg string, flags []string) (err error) {
	q, n, err := Info(pkg)
	if err != nil {
		return
	}

	if n == 0 {
		return fmt.Errorf("Package %s does not exist", pkg)
	}

	q[0].Install(flags)
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
	if !continueTask("Proceed with upgrade?", "n & N") {
		return nil
	}

	for _, pkg := range outdated {
		pkg.Install(flags)
	}

	return nil
}

func (a *Result) setupWorkspace() (err error) {
	// No need to use filepath.separators because it won't run on inferior platforms
	err = os.MkdirAll(BaseDir+"builds", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	tarLocation := BaseDir + a.PackageBase + ".tar.gz"
	defer os.Remove(BaseDir + a.PackageBase + ".tar.gz")

	err = downloadFile(tarLocation, BaseURL+a.URLPath)
	if err != nil {
		return
	}

	err = exec.Command(TarBin, "-xf", tarLocation, "-C", BaseDir).Run()
	if err != nil {
		return
	}

	return
}

// Install handles install from Info Result
func (a *Result) Install(flags []string) (err error) {
	fmt.Printf("\x1b[1;32m==> Installing\x1b[33m %s\x1b[0m\n", a.Name)
	if a.Maintainer == "" {
		fmt.Println("\x1b[1;31;40m==> Warning:\x1b[0;;40m This package is orphaned.\x1b[0m")
	}
	dir := BaseDir + a.PackageBase + "/"

	if _, err = os.Stat(dir); os.IsNotExist(err) {
		if err = a.setupWorkspace(); err != nil {
			return
		}
	}

	// defer os.RemoveAll(BaseDir + a.PackageBase)

	if !continueTask("Edit PKGBUILD?", "y & Y") {
		editcmd := exec.Command(Editor, dir+"PKGBUILD")
		editcmd.Stdin, editcmd.Stdout, editcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		editcmd.Run()
	}

	aurDeps, repoDeps, err := a.Dependencies()
	if err != nil {
		return
	}

	printDependencies(aurDeps, repoDeps)

	if len(aurDeps) != 0 || len(repoDeps) != 0 {
		if !continueTask("Continue?", "n & N") {
			return fmt.Errorf("user did not like the dependencies")
		}
	}

	aurQ, n, err := MultiInfo(aurDeps)
	if n != len(aurDeps) {
		MissingPackage(aurDeps, aurQ)
		if !continueTask("Continue?", "n & N") {
			return fmt.Errorf("unable to install dependencies")
		}
	}

	// Handle AUR dependencies first
	for _, dep := range aurQ {
		errA := dep.Install([]string{"--asdeps", "--noconfirm"})
		if errA != nil {
			return errA
		}
	}

	// Repo dependencies
	if len(repoDeps) != 0 {
		errR := pacman.Install(repoDeps, []string{"--asdeps", "--noconfirm"})
		if errR != nil {
			pacman.CleanRemove(aurDeps)
			return errR
		}
	}

	err = os.Chdir(dir)
	if err != nil {
		return
	}

	var makepkgcmd *exec.Cmd
	var args []string
	args = append(args, "-sri")
	args = append(args, flags...)
	makepkgcmd = exec.Command(MakepkgBin, args...)
	makepkgcmd.Stdin, makepkgcmd.Stdout, makepkgcmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = makepkgcmd.Run()
	return
}

func continueTask(s string, def string) (cont bool) {
	if NoConfirm {
		return true
	}
	var postFix string

	if def == "n & N" {
		postFix = "(Y/n)"
	} else {
		postFix = "(y/N)"
	}

	var response string
	fmt.Printf("\x1b[1;32m==> %s\x1b[1;37m %s\x1b[0m\n", s, postFix)

	fmt.Scanln(&response)
	if strings.ContainsAny(response, def) {
		return false
	}

	return true
}

func printDependencies(aurDeps []string, repoDeps []string) {
	if len(repoDeps) != 0 {
		fmt.Print("\x1b[1;32m==> Repository dependencies: \x1b[0m")
		for _, repoD := range repoDeps {
			fmt.Print("\x1b[33m", repoD, " \x1b[0m")
		}
		fmt.Print("\n")

	}
	if len(repoDeps) != 0 {
		fmt.Print("\x1b[1;32m==> AUR dependencies: \x1b[0m")
		for _, aurD := range aurDeps {
			fmt.Print("\x1b[33m", aurD, " \x1b[0m")
		}
		fmt.Print("\n")
	}
}

// MissingPackage warns if the Query was unable to find a package
func MissingPackage(aurDeps []string, aurQ Query) {
	for _, depName := range aurDeps {
		found := false
		for _, dep := range aurQ {
			if dep.Name == depName {
				found = true
				break
			}
		}

		if !found {
			fmt.Println("\x1b[31mUnable to find", depName, "in AUR\x1b[0m")
		}
	}
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
