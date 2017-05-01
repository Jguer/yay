package github

import "strings"

// Branches contains the information of a repository branch
type Branches []struct {
	Name   string `json:"name"`
	Commit struct {
		Sha string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

const repoAPI = "https://api.github.com/repos/{USER}/{REPOSITORY}/branches"

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
