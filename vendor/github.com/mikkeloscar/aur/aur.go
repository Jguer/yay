package aur

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

//AURURL is the base string from which the query is built
var AURURL = "https://aur.archlinux.org/rpc.php?"

type response struct {
	Error       string `json:"error"`
	Version     int    `json:"version"`
	Type        string `json:"type"`
	ResultCount int    `json:"resultcount"`
	Results     []Pkg  `json:"results"`
}

// Pkg holds package information
type Pkg struct {
	ID             int      `json:"ID"`
	Name           string   `json:"Name"`
	PackageBaseID  int      `json:"PackageBaseID"`
	PackageBase    string   `json:"PackageBase"`
	Version        string   `json:"Version"`
	Description    string   `json:"Description"`
	URL            string   `json:"URL"`
	NumVotes       int      `json:"NumVotes"`
	Popularity     float64  `json:"Popularity"`
	OutOfDate      int      `json:"OutOfDate"`
	Maintainer     string   `json:"Maintainer"`
	FirstSubmitted int      `json:"FirstSubmitted"`
	LastModified   int      `json:"LastModified"`
	URLPath        string   `json:"URLPath"`
	Depends        []string `json:"Depends"`
	MakeDepends    []string `json:"MakeDepends"`
	CheckDepends   []string `json:"CheckDepends"`
	Conflicts      []string `json:"Conflicts"`
	Provides       []string `json:"Provides"`
	Replaces       []string `json:"Replaces"`
	OptDepends     []string `json:"OptDepends"`
	Groups         []string `json:"Groups"`
	License        []string `json:"License"`
	Keywords       []string `json:"Keywords"`
}

func get(values url.Values) ([]Pkg, error) {
	values.Set("v", "5")
	resp, err := http.Get(AURURL + values.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	result := new(response)
	err = dec.Decode(result)
	if err != nil {
		return nil, err
	}

	if len(result.Error) > 0 {
		return nil, errors.New(result.Error)
	}

	return result.Results, nil
}

// Search searches for packages by package name.
func Search(query string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("type", "search")
	v.Set("arg", query)

	return get(v)
}

// SearchByNameDesc searches for package by package name and description.
func SearchByNameDesc(query string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("type", "search")
	v.Set("by", "name-desc")
	v.Set("arg", query)

	return get(v)
}

// SearchByMaintainer searches for package by maintainer.
func SearchByMaintainer(query string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("type", "search")
	v.Set("by", "maintainer")
	v.Set("arg", query)

	return get(v)
}

// Info shows info for one or multiple packages.
func Info(pkgs []string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("type", "info")
	for _, arg := range pkgs {
		v.Add("arg[]", arg)
	}
	return get(v)
}
