package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
)

// RepoResult describes a Repository package
type RepoResult struct {
	Description string
	Repository  string
	Version     string
	Name        string
}

// RepoSearch describes a Repository search
type RepoSearch struct {
	Resultcount int
	Results     []RepoResult
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
}

// AurSearch describes an AUR search
type AurSearch struct {
	Resultcount int         `json:"resultcount"`
	Results     []AurResult `json:"results"`
	Type        string      `json:"type"`
	Version     int         `json:"version"`
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

func (r AurSearch) Len() int {
	return len(r.Results)
}

func (r AurSearch) Less(i, j int) bool {
	return r.Results[i].NumVotes > r.Results[j].NumVotes
}

func (r AurSearch) Swap(i, j int) {
	r.Results[i], r.Results[j] = r.Results[j], r.Results[i]
}

func isInRepo(pkg string) bool {
	if _, err := exec.Command(PacmanBin, "-Sp", pkg).Output(); err != nil {
		return false
	}
	return true
}

func isInstalled(pkg string) bool {
	if _, err := exec.Command(PacmanBin, "-Qq", pkg).Output(); err != nil {
		return false
	}
	return true
}
