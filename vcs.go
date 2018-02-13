package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
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

// createDevelDB forces yay to create a DB of the existing development packages
func createDevelDB() error {
	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	config.NoConfirm = true
	arguments := makeArguments()
	arguments.addArg("gendb")
	arguments.addTarget(remoteNames...)
	err = install(arguments)
	return err
}

// parseSource returns owner and repo from source
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

	body, _ := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(body, &newRepo)
	if err != nil {
		fmt.Printf("Cannot update '%v'\nError: %v\nStatus code: %v\nBody: %v\n",
			info.Package, err, r.StatusCode, string(body))
		return false
	}

	for _, e := range newRepo {
		if e.Name == "master" {
			return e.Commit.SHA != info.SHA
		}
	}
	return false
}

func inStore(pkgName string) *Info {
	for i, e := range savedInfo {
		if pkgName == e.Package {
			return &savedInfo[i]
		}
	}
	return nil
}

// branchInfo updates saved information
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
	in, err := os.OpenFile(vcsFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
