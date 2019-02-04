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

func searchBy(query, by string) ([]Pkg, error) {
	v := url.Values{}
	v.Set("type", "search")
	v.Set("arg", query)

	if by != "" {
		v.Set("by", by)
	}

	return get(v)
}

// Search searches for packages by the RPC's default defautl field.
// This is the same as SearchByNameDesc
func Search(query string) ([]Pkg, error) {
	return searchBy(query, "")
}

// Search searches for packages by package name.
func SearchByName(query string) ([]Pkg, error) {
	return searchBy(query, "name")
}

// SearchByNameDesc searches for package by package name and description.
func SearchByNameDesc(query string) ([]Pkg, error) {
	return searchBy(query, "name-desc")
}

// SearchByMaintainer searches for package by maintainer.
func SearchByMaintainer(query string) ([]Pkg, error) {
	return searchBy(query, "maintainer")
}

// SearchByDepends searches for packages that depend on query
func SearchByDepends(query string) ([]Pkg, error) {
	return searchBy(query, "depends")
}

// SearchByMakeDepends searches for packages that makedepend on query
func SearchByMakeDepends(query string) ([]Pkg, error) {
	return searchBy(query, "makedepends")
}

// SearchByOptDepends searches for packages that optdepend on query
func SearchByOptDepends(query string) ([]Pkg, error) {
	return searchBy(query, "optdepends")
}

// SearchByCheckDepends searches for packages that checkdepend on query
func SearchByCheckDepends(query string) ([]Pkg, error) {
	return searchBy(query, "checkdepends")
}

// Orphans returns all orphan packages in the AUR.
func Orphans() ([]Pkg, error) {
	return SearchByMaintainer("")
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
