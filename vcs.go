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

// Repo contains information about the repository
type repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

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
	var newRepo repo
	var newBranches branches
	if strings.HasSuffix(info.URL, "/branches") {
		info.URL = info.URL[:len(info.URL)-9]
	}
	infoResp, infoErr := http.Get(info.URL)
	if infoErr != nil {
		fmt.Println(infoErr)
		return false
	}
	defer infoResp.Body.Close()

	infoBody, _ := ioutil.ReadAll(infoResp.Body)
	var err = json.Unmarshal(infoBody, &newRepo)
	if err != nil {
		fmt.Printf("Cannot update '%v'\nError: %v\nStatus code: %v\nBody: %v\n",
			info.Package, err, infoResp.StatusCode, string(infoBody))
		return false
	}

	defaultBranch := newRepo.DefaultBranch
	branchesURL := info.URL + "/branches"

	branchResp, branchErr := http.Get(branchesURL)
	if branchErr != nil {
		fmt.Println(branchErr)
		return false
	}
	defer branchResp.Body.Close()

	branchBody, _ := ioutil.ReadAll(branchResp.Body)
	err = json.Unmarshal(branchBody, &newBranches)
	if err != nil {
		fmt.Printf("Cannot update '%v'\nError: %v\nStatus code: %v\nBody: %v\n",
			info.Package, err, branchResp.StatusCode, string(branchBody))
		return false
	}

	for _, e := range newBranches {
		if e.Name == defaultBranch {
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
func branchInfo(pkgName string, owner string, repoName string) (err error) {
	updated := false
	var newRepo repo
	var newBranches branches
	url := "https://api.github.com/repos/" + owner + "/" + repoName
	repoResp, err := http.Get(url)
	if err != nil {
		return
	}
	defer repoResp.Body.Close()

	_ = json.NewDecoder(repoResp.Body).Decode(&newRepo)
	defaultBranch := newRepo.DefaultBranch
	branchesURL := url + "/branches"

	branchResp, err := http.Get(branchesURL)
	if err != nil {
		return
	}
	defer branchResp.Body.Close()

	_ = json.NewDecoder(branchResp.Body).Decode(&newBranches)

	packinfo := inStore(pkgName)

	for _, e := range newBranches {
		if e.Name == defaultBranch {
			updated = true

			if packinfo != nil {
				packinfo.Package = pkgName
				packinfo.URL = url
				packinfo.SHA = e.Commit.SHA
			} else {
				savedInfo = append(savedInfo, Info{Package: pkgName, URL: url, SHA: e.Commit.SHA})
			}
		}
	}

	if updated {
		saveVCSInfo()
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
