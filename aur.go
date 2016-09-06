package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
)

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

func searchAurPackages(pkg string) (search AurSearch) {
	getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkg, &search)
	sort.Sort(search)
	return search
}

func (r AurSearch) printSearch(index int) (err error) {
	for i, result := range r.Results {
		if index != SearchMode {
			fmt.Printf("%d aur/\x1B[33m%s\033[0m \x1B[36m%s\033[0m (%d)\n    %s\n",
				i+index, result.Name, result.Version, result.NumVotes, result.Description)
		} else {
			fmt.Printf("aur/\x1B[33m%s\033[0m \x1B[36m%s\033[0m (%d)\n    %s\n",
				result.Name, result.Version, result.NumVotes, result.Description)
		}
	}

	return
}

func (r AurSearch) installAurArray(num []int, index int) (err error) {
	if len(num) == 0 {
		return nil
	}

	for _, i := range num {
		fmt.Printf("%+v\n\n", r.Results[i-index])
		err = r.Results[i-index].installResult()
		if err != nil {
			fmt.Println(err)
			return err
		}
	}

	return err
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

func (a AurResult) getAURDependencies() {
	return
}

func (a AurResult) installResult() (err error) {
	// No need to use filepath.separators because it won't run on inferior platforms
	err = os.MkdirAll(BuildDir+"builds", 0755)
	if err != nil {
		fmt.Println(err)
		return
	}

	tarLocation := BuildDir + a.Name + ".tar.gz"
	// err = os.MkdirAll(BuildDir+a.Name, 0755)
	// if err != nil {
	// 	return
	// }

	err = downloadFile(tarLocation, BaseURL+a.URLPath)
	if err != nil {
		return
	}

	err = exec.Command(TarBin, "-xf", tarLocation, "-C", BuildDir).Run()
	if err != nil {
		return
	}

	err = os.Chdir(BuildDir + a.Name)
	if err != nil {
		return
	}
	a.getAURDependencies()

	fmt.Print("==> Edit PKGBUILD? (y/n)")
	var response string
	fmt.Scanln(&response)
	if strings.ContainsAny(response, "y & Y") {
		editcmd := exec.Command(Editor, BuildDir+a.Name+"/"+"PKGBUILD")
		editcmd.Stdout = os.Stdout
		editcmd.Stderr = os.Stderr
		editcmd.Stdin = os.Stdin
		err = editcmd.Run()
	}

	makepkgcmd := exec.Command(MakepkgBin, "-sri")
	makepkgcmd.Stdout = os.Stdout
	makepkgcmd.Stderr = os.Stderr
	makepkgcmd.Stdin = os.Stdin
	err = makepkgcmd.Run()

	return
}
