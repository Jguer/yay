package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/text"
)

// InfoStore is a collection of OriginInfoByURL by Package.
// Containing a map of last commit SHAs of a repo.
type InfoStore struct {
	OriginsByPackage map[string]OriginInfoByURL
	FilePath         string
	CmdBuilder       exe.GitCmdBuilder
}

// OriginInfoByURL stores the OriginInfo of each origin URL provided.
type OriginInfoByURL map[string]OriginInfo

// OriginInfo contains the last commit sha of a repo
// Example:
//
//	"github.com/Jguer/yay.git": {
//		"protocols": [
//			"https"
//		],
//		"branch": "next",
//		"sha": "c1171d41467c68ffd3c46748182a16366aaaf87b"
//	}.
type OriginInfo struct {
	Protocols []string `json:"protocols"`
	Branch    string   `json:"branch"`
	SHA       string   `json:"sha"`
}

func NewInfoStore(filePath string, cmdBuilder exe.GitCmdBuilder) *InfoStore {
	infoStore := &InfoStore{
		CmdBuilder:       cmdBuilder,
		FilePath:         filePath,
		OriginsByPackage: map[string]OriginInfoByURL{},
	}

	return infoStore
}

// GetCommit parses HEAD commit from url and branch.
func (v *InfoStore) getCommit(ctx context.Context, url, branch string, protocols []string) string {
	if len(protocols) > 0 {
		protocol := protocols[len(protocols)-1]

		ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		cmd := v.CmdBuilder.BuildGitCmd(ctxTimeout, "", "ls-remote", protocol+"://"+url, branch)

		stdout, _, err := v.CmdBuilder.Capture(cmd)
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok && exiterr.ExitCode() == 128 {
				text.Warnln(gotext.Get("devel check for package failed: '%s' encountered an error", cmd.String()))
				return ""
			}

			text.Warnln(err)

			return ""
		}

		split := strings.Fields(stdout)

		if len(split) < 2 {
			return ""
		}

		commit := split[0]

		return commit
	}

	return ""
}

func (v *InfoStore) Update(ctx context.Context, pkgName string,
	sources []gosrc.ArchString, mux sync.Locker, wg *sync.WaitGroup,
) {
	defer wg.Done()

	info := make(OriginInfoByURL)
	checkSource := func(source gosrc.ArchString) {
		defer wg.Done()

		url, branch, protocols := parseSource(source.Value)
		if url == "" || branch == "" {
			return
		}

		commit := v.getCommit(ctx, url, branch, protocols)
		if commit == "" {
			return
		}

		mux.Lock()
		info[url] = OriginInfo{
			protocols,
			branch,
			commit,
		}

		v.OriginsByPackage[pkgName] = info

		text.Warnln(gotext.Get("Found git repo: %s", text.Cyan(url)))

		if err := v.Save(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		mux.Unlock()
	}

	for _, source := range sources {
		wg.Add(1)

		go checkSource(source)
	}
}

// parseSource returns the git url, default branch and protocols it supports.
func parseSource(source string) (url, branch string, protocols []string) {
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
			// source has #commit= or #tag= which makes them not vcs
			// packages because they reference a specific point
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

	return url, branch, protocols
}

func (v *InfoStore) NeedsUpdate(ctx context.Context, infos OriginInfoByURL) bool {
	// used to signal we have gone through all sources and found nothing
	finished := make(chan struct{})
	alive := 0

	// if we find an update we use this to exit early and return true
	hasUpdate := make(chan struct{})

	closed := make(chan struct{})
	defer close(closed)

	checkHash := func(url string, info OriginInfo) {
		hash := v.getCommit(ctx, url, info.Branch, info.Protocols)

		var sendTo chan<- struct{}
		if hash != "" && hash != info.SHA {
			sendTo = hasUpdate
		} else {
			sendTo = finished
		}

		select {
		case sendTo <- struct{}{}:
		case <-closed:
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

func (v *InfoStore) Save() error {
	marshalledinfo, err := json.MarshalIndent(v.OriginsByPackage, "", "\t")
	if err != nil || string(marshalledinfo) == "null" {
		return err
	}

	in, err := os.OpenFile(v.FilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	defer in.Close()

	if _, errM := in.Write(marshalledinfo); errM != nil {
		return errM
	}

	return in.Sync()
}

// RemovePackage removes package from VCS information.
func (v *InfoStore) RemovePackage(pkgs []string) {
	updated := false

	for _, pkgName := range pkgs {
		if _, ok := v.OriginsByPackage[pkgName]; ok {
			delete(v.OriginsByPackage, pkgName)

			updated = true
		}
	}

	if updated {
		if err := v.Save(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

// LoadStore reads a json file and populates a InfoStore structure.
func (v InfoStore) Load() error {
	vfile, err := os.Open(v.FilePath)
	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("failed to open vcs file '%s': %s", v.FilePath, err)
	}

	defer vfile.Close()

	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&v.OriginsByPackage); err != nil {
			return fmt.Errorf("failed to read vcs '%s': %s", v.FilePath, err)
		}
	}

	return nil
}
