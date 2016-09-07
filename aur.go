package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// AurInfo is the result of an info search
type AurInfo struct {
	Version     int    `json:"version"`
	Type        string `json:"type"`
	Resultcount int    `json:"resultcount"`
	Results     []struct {
		ID             int         `json:"ID"`
		Name           string      `json:"Name"`
		PackageBaseID  int         `json:"PackageBaseID"`
		PackageBase    string      `json:"PackageBase"`
		Version        string      `json:"Version"`
		Description    string      `json:"Description"`
		URL            string      `json:"URL"`
		NumVotes       int         `json:"NumVotes"`
		Popularity     float64     `json:"Popularity"`
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
	} `json:"results"`
}

// AurResult describes an AUR package
type AurResult struct {
	ID             int         `json:"ID"`
	Name           string      `json:"Name"`
	PackageBaseID  int         `json:"PackageBaseID"`
	PackageBase    string      `json:"PackageBase"`
	Version        string      `json:"Version"`
	Description    string      `json:"Description"`
	URL            string      `json:"URL"`
	NumVotes       int         `json:"NumVotes"`
	Popularity     int         `json:"Popularity"`
	OutOfDate      interface{} `json:"OutOfDate"`
	Maintainer     string      `json:"Maintainer"`
	FirstSubmitted int         `json:"FirstSubmitted"`
	LastModified   int         `json:"LastModified"`
	URLPath        string      `json:"URLPath"`
}

// AurSearch describes an AUR search
type AurSearch struct {
	Resultcount int         `json:"resultcount"`
	Results     []AurResult `json:"results"`
	Type        string      `json:"type"`
	Version     int         `json:"version"`
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

func (r AurSearch) Len() int {
	return len(r.Results)
}

func (r AurSearch) Less(i, j int) bool {
	return r.Results[i].NumVotes > r.Results[j].NumVotes
}

func (r AurSearch) Swap(i, j int) {
	r.Results[i], r.Results[j] = r.Results[j], r.Results[i]
}

func searchAurPackages(pkg string) (search AurSearch, err error) {
	err = getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkg, &search)
	sort.Sort(search)
	return
}

func infoAurPackage(pkg string) (info AurInfo, err error) {
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

func installAURPackage(pkgList string) (err error) {
	return err
}

func (a AurResult) getAURDependencies() (err error) {
	_, err = a.getDepsFromRPC()

	return nil
}

func (a AurResult) installResult() (err error) {
	// No need to use filepath.separators because it won't run on inferior platforms
	err = os.MkdirAll(BuildDir+"builds", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	tarLocation := BuildDir + a.Name + ".tar.gz"

	err = downloadFile(tarLocation, BaseURL+a.URLPath)
	if err != nil {
		return
	}

	err = exec.Command(TarBin, "-xf", tarLocation, "-C", BuildDir).Run()
	if err != nil {
		return
	}

	a.getAURDependencies()
	os.Exit(0)

	fmt.Print("\033[1m\x1b[32m==> Edit PKGBUILD? (y/n)\033[0m")
	var response string
	fmt.Scanln(&response)
	if strings.ContainsAny(response, "y & Y") {
		editcmd := exec.Command(Editor, BuildDir+a.Name+"/"+"PKGBUILD")
		editcmd.Stdout = os.Stdout
		editcmd.Stderr = os.Stderr
		editcmd.Stdin = os.Stdin
		err = editcmd.Run()
	}

	err = os.Chdir(BuildDir + a.Name)
	if err != nil {
		return
	}

	makepkgcmd := exec.Command(MakepkgBin, "-sri")
	makepkgcmd.Stdout = os.Stdout
	makepkgcmd.Stderr = os.Stderr
	makepkgcmd.Stdin = os.Stdin
	err = makepkgcmd.Run()

	return
}
