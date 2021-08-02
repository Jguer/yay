package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"
)

const gitDiffRefName = "AUR_SEEN"

// Update the YAY_DIFF_REVIEW ref to HEAD. We use this ref to determine which diff were
// reviewed by the user
func gitUpdateSeenRef(path, name string) error {
	_, stderr, err := config.Runtime.CmdRunner.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(
			filepath.Join(path, name), "update-ref", gitDiffRefName, "HEAD"), 0)
	if err != nil {
		return fmt.Errorf("%s %s", stderr, err)
	}
	return nil
}

// Return wether or not we have reviewed a diff yet. It checks for the existence of
// YAY_DIFF_REVIEW in the git ref-list
func gitHasLastSeenRef(path, name string) bool {
	_, _, err := config.Runtime.CmdRunner.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(
			filepath.Join(path, name), "rev-parse", "--quiet", "--verify", gitDiffRefName), 0)
	return err == nil
}

// Returns the last reviewed hash. If YAY_DIFF_REVIEW exists it will return this hash.
// If it does not it will return empty tree as no diff have been reviewed yet.
func getLastSeenHash(path, name string) (string, error) {
	if gitHasLastSeenRef(path, name) {
		stdout, stderr, err := config.Runtime.CmdRunner.Capture(
			config.Runtime.CmdBuilder.BuildGitCmd(
				filepath.Join(path, name), "rev-parse", gitDiffRefName), 0)
		if err != nil {
			return "", fmt.Errorf("%s %s", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		return lines[0], nil
	}
	return gitEmptyTree, nil
}

// Check whether or not a diff exists between the last reviewed diff and
// HEAD@{upstream}
func gitHasDiff(path, name string) (bool, error) {
	if gitHasLastSeenRef(path, name) {
		stdout, stderr, err := config.Runtime.CmdRunner.Capture(
			config.Runtime.CmdBuilder.BuildGitCmd(filepath.Join(path, name), "rev-parse", gitDiffRefName, "HEAD@{upstream}"), 0)
		if err != nil {
			return false, fmt.Errorf("%s%s", stderr, err)
		}

		lines := strings.Split(stdout, "\n")
		lastseen := lines[0]
		upstream := lines[1]
		return lastseen != upstream, nil
	}
	// If YAY_DIFF_REVIEW does not exists, we have never reviewed a diff for this package
	// and should display it.
	return true, nil
}

func gitDownload(url, path, name string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		cmd := config.Runtime.CmdBuilder.BuildGitCmd(path, "clone", "--no-progress", url, name)
		_, stderr, errCapture := config.Runtime.CmdRunner.Capture(cmd, 0)
		if errCapture != nil {
			return false, fmt.Errorf(gotext.Get("error cloning %s: %s", name, stderr))
		}

		return true, nil
	} else if err != nil {
		return false, fmt.Errorf(gotext.Get("error reading %s", filepath.Join(path, name, ".git")))
	}

	cmd := config.Runtime.CmdBuilder.BuildGitCmd(filepath.Join(path, name), "fetch")
	_, stderr, err := config.Runtime.CmdRunner.Capture(cmd, 0)
	if err != nil {
		return false, fmt.Errorf(gotext.Get("error fetching %s: %s", name, stderr))
	}

	return false, nil
}

func gitMerge(path, name string) error {
	_, stderr, err := config.Runtime.CmdRunner.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(
			filepath.Join(path, name), "reset", "--hard", "HEAD"), 0)
	if err != nil {
		return fmt.Errorf(gotext.Get("error resetting %s: %s", name, stderr))
	}

	_, stderr, err = config.Runtime.CmdRunner.Capture(
		config.Runtime.CmdBuilder.BuildGitCmd(
			filepath.Join(path, name), "merge", "--no-edit", "--ff"), 0)
	if err != nil {
		return fmt.Errorf(gotext.Get("error merging %s: %s", name, stderr))
	}

	return nil
}
