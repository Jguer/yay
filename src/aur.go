package main

import (
	"encoding/json"
	"fmt"
	c "github.com/fatih/color"
	"net/http"
	"sort"
)

// AurResult describes an AUR package
type AurResult struct {
	Description    string      `json:"Description"`
	FirstSubmitted int         `json:"FirstSubmitted"`
	ID             int         `json:"ID"`
	LastModified   int         `json:"LastModified"`
	Maintainer     string      `json:"Maintainer"`
	Name           string      `json:"Name"`
	NumVotes       int         `json:"NumVotes"`
	OutOfDate      interface{} `json:"OutOfDate"`
	PackageBase    string      `json:"PackageBase"`
	PackageBaseID  int         `json:"PackageBaseID"`
	Popularity     int         `json:"Popularity"`
	URL            string      `json:"URL"`
	URLPath        string      `json:"URLPath"`
	Version        string      `json:"Version"`
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

// AurSearch describes an AUR search
type AurSearch struct {
	Resultcount int         `json:"resultcount"`
	Results     []AurResult `json:"results"`
	Type        string      `json:"type"`
	Version     int         `json:"version"`
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
	yellow := c.New(c.FgYellow).SprintFunc()
	green := c.New(c.FgGreen).SprintFunc()

	for i, result := range r.Results {
		if index != SearchMode {
			fmt.Printf("%d aur/%s %s (%d)\n    %s\n", i+index, yellow(result.Name), green(result.Version), result.NumVotes, result.Description)
		} else {
			fmt.Printf("aur/%s %s (%d)\n    %s\n", yellow(result.Name), green(result.Version), result.NumVotes, result.Description)
		}
	}

	return
}

func (a AurResult) installResult() error {
	return nil
}
