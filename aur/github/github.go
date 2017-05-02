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
	Name string `json:"name"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

type branches []branch

// Info contains the last commit sha of a repo
type Info struct {
	Package string `json:"owner"`
	Repo    string `json:"Repo"`
	SHA     string `json:"sha"`
}

type infos []Info

var savedInfo infos

func init() {
	path := os.Getenv("HOME") + "/.config/yay/yay_github.json"

	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(os.Getenv("HOME")+"/.config/yay/yay_github.json", 0755)
	}

	file, err := os.Open(path)
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

func parseSource(source string) (owner string, repo string) {
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

func checkUpdates() {

}

func inStore(pkgname string) *Info {
	for i, e := range savedInfo {
		if pkgname == e.Package {
			return &savedInfo[i]
		}
	}
	return nil
}

// BranchInfo updates saved information
func BranchInfo(pkgname string, owner string, repo string) (err error) {
	var newRepo branches
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/branches"
	r, err := http.Get(url)
	if err != nil {
		return
	}
	defer r.Body.Close()

	json.NewDecoder(r.Body).Decode(newRepo)

	packinfo := inStore(pkgname)

	for _, e := range newRepo {
		if e.Name == "master" {
			if packinfo != nil {
				packinfo.Package = pkgname
				packinfo.Repo = owner + "/" + repo
				packinfo.SHA = e.SHA
			} else {
				savedInfo = append(savedInfo, Info{Package: pkgname, Repo: owner + "/" + repo, SHA: e.SHA})
			}
		}
	}

	return
}
