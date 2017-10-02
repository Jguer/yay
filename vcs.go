package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	alpm "github.com/jguer/go-alpm"
)

// branch contains the information of a repository branch
type branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

type branches []branch

// Info contains the last commit sha of a repo
type Info struct {
	Package string `json:"pkgname"`
	URL     string `json:"url"`
	SHA     string `json:"sha"`
}

type infos []Info

// CreateDevelDB forces yay to create a DB of the existing development packages
func createDevelDB() error {
	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	config.NoConfirm = true
	specialDBsauce = true
	err = aurInstall(remoteNames, nil)
	return err
}

// ParseSource returns owner and repo from source
func parseSource(source string) (owner string, repo string) {
	if !(strings.Contains(source, "git://") ||
		strings.Contains(source, ".git") ||
		strings.Contains(source, "git+https://")) {
		return
	}
	split := strings.Split(source, "github.com/")
	if len(split) > 1 {
		secondSplit := strings.Split(split[1], "/")
		if len(secondSplit) > 1 {
			owner = secondSplit[0]
			thirdSplit := strings.Split(secondSplit[1], ".git")
			if len(thirdSplit) > 0 {
				repo = thirdSplit[0]
			}
		}
	}
	return
}

func (info *Info) needsUpdate() bool {
	var newRepo branches
	r, err := http.Get(info.URL)
	if err != nil {
		fmt.Println(err)
		return false
	}
	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(&newRepo)
	if err != nil {
		fmt.Println(err)
		return false
	}

	for _, e := range newRepo {
		if e.Name == "master" {
			if e.Commit.SHA != info.SHA {
				return true
			}
			return false
		}
	}
	return false
}

// CheckUpdates returns list of outdated packages
func checkUpdates(foreign map[string]alpm.Package) (toUpdate []string) {
	for _, e := range savedInfo {
		if e.needsUpdate() {
			if _, ok := foreign[e.Package]; ok {
				toUpdate = append(toUpdate, e.Package)
			} else {
				removeVCSPackage([]string{e.Package})
			}
		}
	}
	return
}

func inStore(pkgName string) *Info {
	for i, e := range savedInfo {
		if pkgName == e.Package {
			return &savedInfo[i]
		}
	}
	return nil
}

// BranchInfo updates saved information
func branchInfo(pkgName string, owner string, repo string) (err error) {
	updated = true
	var newRepo branches
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/branches"
	r, err := http.Get(url)
	if err != nil {
		return
	}
	defer r.Body.Close()

	_ = json.NewDecoder(r.Body).Decode(&newRepo)

	packinfo := inStore(pkgName)

	for _, e := range newRepo {
		if e.Name == "master" {
			if packinfo != nil {
				packinfo.Package = pkgName
				packinfo.URL = url
				packinfo.SHA = e.Commit.SHA
			} else {
				savedInfo = append(savedInfo, Info{Package: pkgName, URL: url, SHA: e.Commit.SHA})
			}
		}
	}

	return
}

func saveVCSInfo() error {
	marshalledinfo, err := json.MarshalIndent(savedInfo, "", "\t")
	if err != nil || string(marshalledinfo) == "null" {
		return err
	}
	in, err := os.OpenFile(configfile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = in.Write(marshalledinfo)
	if err != nil {
		return err
	}
	err = in.Sync()
	return err
}
