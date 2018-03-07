package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Info contains the last commit sha of a repo
type vcsInfo map[string]shaInfos
type shaInfos map[string]shaInfo
type shaInfo struct {
	Protocols []string `json:"protocols"`
	Brach     string   `json:"branch"`
	SHA       string   `json:"sha"`
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

// parseSource returns the git url, default branch and protocols it supports
func parseSource(source string) (url string, branch string, protocols []string) {
	if !(strings.Contains(source, "git://") ||
		strings.Contains(source, ".git") ||
		strings.Contains(source, "git+https://")) {
		return "", "", nil
	}
	split := strings.Split(source, "::")
	source = split[len(split)-1]
	split = strings.SplitN(source, "://", 2)

	if len(split) != 2 {
		return "", "", nil
	}

	protocols = strings.Split(split[0], "+")
	split = strings.SplitN(split[1], "#", 2)
	if len(split) == 2 {
		secondSplit := strings.SplitN(split[1], "=", 2)
		if secondSplit[0] != "branch" {
			//source has #commit= or #tag= which makes them not vcs
			//packages because they reference a specific point
			return "", "", nil
		}

		if len(secondSplit) == 2 {
			url = split[0]
			branch = secondSplit[1]
		}
	} else {
		url = split[0]
		branch = "HEAD"
	}

	return
}

func updateVCSData(pkgName string, sources []string) {
	if savedInfo == nil {
		savedInfo = make(vcsInfo)
	}

	info := make(shaInfos)

	for _, source := range sources {
		url, branch, protocols := parseSource(source)
		if url == "" || branch == "" {
			continue
		}

		commit := getCommit(url, branch, protocols)
		if commit == "" {
			continue
		}

		info[url] = shaInfo{
			protocols,
			branch,
			commit,
		}

		savedInfo[pkgName] = info
		saveVCSInfo()
	}
}

func getCommit(url string, branch string, protocols []string) string {
	for _, protocol := range protocols {
		var outbuf bytes.Buffer

		cmd := exec.Command(config.GitBin, "ls-remote", protocol+"://"+url, branch)
		cmd.Stdout = &outbuf

		err := cmd.Start()
		if err != nil {
			continue
		}

		//for some reason
		//git://bitbucket.org/volumesoffun/polyvox.git` hangs on my
		//machine but using http:// instead of git does not hang.
		//Introduce a time out so this can not hang
		timer := time.AfterFunc(5*time.Second, func() {
			cmd.Process.Kill()
		})

		err = cmd.Wait()
		timer.Stop()

		if err != nil {
			continue
		}
		err = cmd.Run()

		stdout := outbuf.String()
		split := strings.Fields(stdout)

		if len(split) < 2 {
			continue
		}

		commit := split[0]
		return commit
	}

	return ""
}

func (infos shaInfos) needsUpdate() bool {
	for url, info := range infos {
		hash := getCommit(url, info.Brach, info.Protocols)
		if hash != "" && hash != info.SHA {
			return true
		}
	}

	return false
}

func inStore(pkgName string) shaInfos {
	return savedInfo[pkgName]
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
