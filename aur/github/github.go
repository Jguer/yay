package github

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Branch contains the information of a repository branch
type Branch struct {
	Name   string `json:"name"`
	Commit struct {
		Sha string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
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

func branchInfo(owner string, repo string) (newRepo []Branch, err error) {
	url := "https://api.github.com/repos/" + owner + "/" + repo + "/branches"
	r, err := http.Get(url)
	if err != nil {
		return
	}
	defer r.Body.Close()

	json.NewDecoder(r.Body).Decode(newRepo)

	return
}
