package github

import (
	"encoding/json"
	"fmt"
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

var savedInfo infos
var configfile string

// Updated returns if database has been updated
var Updated bool

func init() {
	Updated = false
	configfile = os.Getenv("HOME") + "/.config/yay/yay_vcs.json"

	if _, err := os.Stat(configfile); os.IsNotExist(err) {
		_ = os.MkdirAll(os.Getenv("HOME")+"/.config/yay", 0755)
		return
	}

	file, err := os.Open(configfile)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&savedInfo)
	if err != nil {
		fmt.Println("error:", err)
	}
}

// ParseSource returns owner and repo from source
func ParseSource(source string) (owner string, repo string) {
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
			} else {
				return false
			}
		}
	}
	return false
}

func CheckUpdates() (toUpdate []string) {
	for _, e := range savedInfo {
		if e.needsUpdate() {
			toUpdate = append(toUpdate, e.Package)
		}
	}
	return
}

func inStore(url string) *Info {
	for i, e := range savedInfo {
		if url == e.URL {
			return &savedInfo[i]
		}
	}
	return nil
}

// BranchInfo updates saved information
func BranchInfo(pkgname string, owner string, repo string) (err error) {
	Updated = true
	var newRepo branches
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/branches"
	r, err := http.Get(url)
	if err != nil {
		return
	}
	defer r.Body.Close()

	_ = json.NewDecoder(r.Body).Decode(&newRepo)

	packinfo := inStore(url)

	for _, e := range newRepo {
		if e.Name == "master" {
			if packinfo != nil {
				packinfo.Package = pkgname
				packinfo.URL = url
				packinfo.SHA = e.Commit.SHA
			} else {
				savedInfo = append(savedInfo, Info{Package: pkgname, URL: url, SHA: e.Commit.SHA})
			}
		}
	}

	return
}

func SaveBranchInfo() error {
	marshalledinfo, _ := json.Marshal(savedInfo)
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
