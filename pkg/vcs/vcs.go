package vcs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	gosrc "github.com/Morganamilo/go-srcinfo"

	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
)

const arrow = "==>"

// InfoStore is a collection of SHAInfos containing a map of last commit shas of a repo
type InfoStore map[string]SHAInfos

type SHAInfos map[string]SHAInfo

// SHAInfo contains the last commit sha of a repo
type SHAInfo struct {
	Protocols []string `json:"protocols"`
	Branch    string   `json:"branch"`
	SHA       string   `json:"sha"`
}

var vcsFile string // To Remove, turn infoStore into a composite structure

// GetCommit parses HEAD commit from url and branch
func getCommit(url string, branch string, protocols []string, config *runtime.Configuration) string {
	if len(protocols) > 0 {
		protocol := protocols[len(protocols)-1]
		var outbuf bytes.Buffer

		cmd := exec.PassToGit(config.GitBin, config.GitFlags, "", "ls-remote", protocol+"://"+url, branch)
		cmd.Stdout = &outbuf
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

		err := cmd.Start()
		if err != nil {
			return ""
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
			return ""
		}

		stdout := outbuf.String()
		split := strings.Fields(stdout)

		if len(split) < 2 {
			return ""
		}

		commit := split[0]
		return commit
	}

	return ""
}

func (v InfoStore) Update(config *runtime.Configuration, pkgName string, sources []gosrc.ArchString, mux *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	if v == nil {
		mux.Lock()
		v = make(InfoStore)
		mux.Unlock()
	}

	info := make(SHAInfos)
	checkSource := func(source gosrc.ArchString) {
		defer wg.Done()
		url, branch, protocols := parseSource(source.Value)
		if url == "" || branch == "" {
			return
		}

		commit := getCommit(url, branch, protocols, config)
		if commit == "" {
			return
		}

		mux.Lock()
		info[url] = SHAInfo{
			protocols,
			branch,
			commit,
		}

		v[pkgName] = info
		fmt.Println(text.Bold(text.Yellow(arrow)) + " Found git repo: " + text.Cyan(url))
		v.Save()
		mux.Unlock()
	}

	for _, source := range sources {
		wg.Add(1)
		go checkSource(source)
	}
}

// parseSource returns the git url, default branch and protocols it supports
func parseSource(source string) (url string, branch string, protocols []string) {
	split := strings.Split(source, "::")
	source = split[len(split)-1]
	split = strings.SplitN(source, "://", 2)

	if len(split) != 2 {
		return "", "", nil
	}
	protocols = strings.SplitN(split[0], "+", 2)

	git := false
	for _, protocol := range protocols {
		if protocol == "git" {
			git = true
			break
		}
	}

	protocols = protocols[len(protocols)-1:]

	if !git {
		return "", "", nil
	}

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

	url = strings.Split(url, "?")[0]
	branch = strings.Split(branch, "?")[0]

	return
}

func (infos SHAInfos) NeedsUpdate(config *runtime.Configuration) bool {
	//used to signal we have gone through all sources and found nothing
	finished := make(chan struct{})
	alive := 0

	//if we find an update we use this to exit early and return true
	hasUpdate := make(chan struct{})

	checkHash := func(url string, info SHAInfo) {
		hash := getCommit(url, info.Branch, info.Protocols, config)
		if hash != "" && hash != info.SHA {
			hasUpdate <- struct{}{}
		} else {
			finished <- struct{}{}
		}
	}

	for url, info := range infos {
		alive++
		go checkHash(url, info)
	}

	for {
		select {
		case <-hasUpdate:
			return true
		case <-finished:
			alive--
			if alive == 0 {
				return false
			}
		}
	}
}

func (v InfoStore) Save() error {
	marshalledinfo, err := json.MarshalIndent(v, "", "\t")
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

// RemovePackage removes package from VCS information
func (v InfoStore) RemovePackage(pkgs []string) {
	updated := false

	for _, pkgName := range pkgs {
		if _, ok := v[pkgName]; ok {
			delete(v, pkgName)
			updated = true
		}
	}

	if updated {
		v.Save()
	}
}

// ReadVCSFromFile reads a json file and returns a InfoStore structure
func ReadVCSFromFile(filePath string) (InfoStore, error) {
	vcsInfo := make(InfoStore)
	vfile, err := os.Open(filePath)
	if !os.IsNotExist(err) && err != nil {
		return nil, fmt.Errorf("Failed to open vcs file '%s': %s", filePath, err)
	}

	defer vfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&vcsInfo); err != nil {
			return nil, fmt.Errorf("Failed to read vcs '%s': %s", filePath, err)
		}
	}

	vcsFile = filePath
	return vcsInfo, nil
}
