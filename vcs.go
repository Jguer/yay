package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	gosrc "github.com/Morganamilo/go-srcinfo"
	rpc "github.com/mikkeloscar/aur"
)

// Info contains the last commit sha of a repo
type vcsInfo map[string]shaInfos
type shaInfos map[string]shaInfo
type shaInfo struct {
	Protocols []string `json:"protocols"`
	Branch    string   `json:"branch"`
	SHA       string   `json:"sha"`
}

// createDevelDB forces yay to create a DB of the existing development packages
func createDevelDB() error {
	var mux sync.Mutex
	var wg sync.WaitGroup
	infoMap := make(map[string]*rpc.Pkg)
	srcinfosStale := make(map[string]*gosrc.Srcinfo)

	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	info, err := aurInfoPrint(remoteNames)
	if err != nil {
		return err
	}

	for _, pkg := range info {
		infoMap[pkg.Name] = pkg
	}

	bases := getBases(infoMap)

	toSkip := pkgBuildsToSkip(info, sliceToStringSet(remoteNames))
	downloadPkgBuilds(info, bases, toSkip)
	tryParsesrcinfosFile(info, srcinfosStale, bases)

	for _, pkgbuild := range srcinfosStale {
		for _, pkg := range pkgbuild.Packages {
			wg.Add(1)
			go updateVCSData(pkg.Pkgname, pkgbuild.Source, &mux, &wg)
		}
	}

	wg.Wait()
	fmt.Println(bold(yellow(arrow) + bold(" GenDB finished. No packages were installed")))
	return err
}

// parseSource returns the git url, default branch and protocols it supports
func parseSource(source string) (url string, branch string, protocols []string) {
	split := strings.Split(source, "::")
	source = split[len(split)-1]
	split = strings.SplitN(source, "://", 2)

	if len(split) != 2 {
		return "", "", nil
	}
	protocols = strings.Split(split[0], "+")

	git := false
	for _, protocol := range protocols {
		if protocol == "git" {
			git = true
			break
		}
	}

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

func updateVCSData(pkgName string, sources []gosrc.ArchString, mux *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()

	if savedInfo == nil {
		mux.Lock()
		savedInfo = make(vcsInfo)
		mux.Unlock()
	}

	info := make(shaInfos)
	checkSource := func(source gosrc.ArchString) {
		defer wg.Done()
		url, branch, protocols := parseSource(source.Value)
		if url == "" || branch == "" {
			return
		}

		commit := getCommit(url, branch, protocols)
		if commit == "" {
			return
		}

		mux.Lock()
		info[url] = shaInfo{
			protocols,
			branch,
			commit,
		}

		savedInfo[pkgName] = info
		fmt.Println(bold(yellow(arrow)) + " Found git repo: " + cyan(url))
		saveVCSInfo()
		mux.Unlock()
	}

	for _, source := range sources {
		wg.Add(1)
		go checkSource(source)
	}
}

func getCommit(url string, branch string, protocols []string) string {
	for _, protocol := range protocols {
		var outbuf bytes.Buffer

		cmd := passToGit("", "ls-remote", protocol+"://"+url, branch)
		cmd.Stdout = &outbuf
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

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
			fmt.Println(bold(yellow(arrow)), "Timeout:", cyan(url))
		})

		err = cmd.Wait()
		timer.Stop()
		if err != nil {
			continue
		}

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
	//used to signal we have gone through all sources and found nothing
	finished := make(chan struct{})
	alive := 0

	//if we find an update we use this to exit early and return true
	hasUpdate := make(chan struct{})

	checkHash := func(url string, info shaInfo) {
		hash := getCommit(url, info.Branch, info.Protocols)
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
