package main

import (
	"fmt"
	c "github.com/fatih/color"
	"os"
	"os/exec"
	"strings"
)

// RepoResult describes a Repository package
type RepoResult struct {
	Description string
	Repository  string
	Version     string
	Name        string
}

// RepoSearch describes a Repository search
type RepoSearch struct {
	Resultcount int
	Results     []RepoResult
}

func getInstalledPackage(pkg string) (err error) {
	cmd := exec.Command(PacmanBin, "-Qi", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return
}

// SearchPackages handles repo searches
func SearchPackages(pkg string) (search RepoSearch, err error) {
	cmd := exec.Command(PacmanBin, "-Ss", pkg)
	cmdOutput, _ := cmd.Output()
	outputSlice := strings.Split(string(cmdOutput), "\n")
	if outputSlice[0] == "" {
		return
	}

	i := true
	var tempStr string
	var rRes *RepoResult
	for _, pkgStr := range outputSlice {
		if i {
			rRes = new(RepoResult)
			fmt.Sscanf(pkgStr, "%s %s\n", &tempStr, &rRes.Version)
			repoNameSlc := strings.Split(tempStr, "/")
			rRes.Repository = repoNameSlc[0]
			rRes.Name = repoNameSlc[1]
			i = false
		} else {
			rRes.Description = pkgStr
			search.Resultcount++
			search.Results = append(search.Results, *rRes)
			i = true
		}
	}
	return
}

func (s RepoSearch) printSearch(index int) (err error) {
	yellow := c.New(c.FgYellow).SprintFunc()
	green := c.New(c.FgGreen).SprintFunc()

	for i, result := range s.Results {
		if index != SearchMode {
			fmt.Printf("%d %s/\x1B[33m%s\033[0m \x1B[36m%s\033[0m\n%s\n",
				i, result.Repository, result.Name, result.Version, result.Description)
		} else {
			fmt.Printf("%s/\x1B[33m%s\033[0m \x1B[36m%s\033[0m\n%s\n",
				result.Repository, yellow(result.Name), green(result.Version), result.Description)
		}
	}

	return nil
}
