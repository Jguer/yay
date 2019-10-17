package aurfetch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

/// The ref used for tracking what commits have been seen
const REF_NAME string = "AUR_SEEN"
const EMPTY_TREE string = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func combinedTrim(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	return outStr, err
}

func (h Handle) gitCommand(dir string, command string, _args ...string) *exec.Cmd {
	args := make([]string, 0, len(h.GitArgs)+2+len(_args)+len(h.GitCommandArgs))
	args = append(args, h.GitArgs...)
	args = append(args, "-C", dir)
	args = append(args, command)
	args = append(args, h.GitCommandArgs...)
	args = append(args, _args...)

	cmd := exec.Command(h.GitCommand, args...)
	if len(h.GitEnvironment) > 0 {
		cmd.Env = append(os.Environ(), h.GitEnvironment...)
	}

	return cmd
}

func (h Handle) gitDownload(url string, path string, name string) (string, bool, error) {
	_, err := os.Stat(filepath.Join(path, name, ".git"))
	if os.IsNotExist(err) {
		cmd := h.gitCommand(path, "clone", "--no-progress", url, name)
		out, err := combinedTrim(cmd)
		if err != nil {
			return string(out), true, fmt.Errorf("error cloning %s: %s: command: %s", name, out, cmd)
		}

		return string(out), true, nil
	} else if err != nil {
		return "", false, fmt.Errorf("error reading %s: %s", filepath.Join(path, name, ".git"), err.Error())
	}

	cmd := h.gitCommand(filepath.Join(path, name), "fetch", "-v")
	out, err := combinedTrim(cmd)
	if err != nil {
		return string(out), false, fmt.Errorf("error fetching %s: %s: command: %s", name, out, cmd)
	}

	return string(out), false, nil
}

func (h Handle) gitMerge(path string, name string) (string, error) {
	var resetRef string

	if h.gitHasRef(path, name) {
		resetRef = REF_NAME
	} else {
		resetRef = "HEAD"
	}

	cmd := h.gitCommand(filepath.Join(path, name), "reset", "--hard", "-q", resetRef)
	out, err := combinedTrim(cmd)
	if err != nil {
		return "", fmt.Errorf("error resetting %s: %s: command: %s", name, out, cmd)
	}

	cmd = h.gitCommand(filepath.Join(path, name), "rebase")
	out, err = combinedTrim(cmd)
	if err != nil {
		return string(out), fmt.Errorf("error merging %s: %s: command: %s", name, out, cmd.String())
	}

	return string(out), nil
}

func (h Handle) gitUpdateRef(path string, name string) error {
	cmd := h.gitCommand(filepath.Join(path, name), "update-ref", REF_NAME, "HEAD")
	out, err := combinedTrim(cmd)
	if err != nil {
		return fmt.Errorf("error updating ref %s: %s: command: %s", name, out, cmd)
	}
	return nil
}

func (h Handle) gitHasRef(path string, name string) bool {
	_, err := h.gitRevParse(filepath.Join(path, name), REF_NAME)
	return err == nil
}

func (h Handle) gitDiff(path string, color bool, name string) (string, error) {
	var diffRef string
	var resetRef string

	if h.gitHasRef(path, name) {
		diffRef = REF_NAME
		resetRef = REF_NAME
	} else {
		diffRef = EMPTY_TREE
		resetRef = "HEAD"
	}

	cmd := h.gitCommand(filepath.Join(path, name), "reset", "--hard", resetRef)
	out, err := combinedTrim(cmd)
	if err != nil {
		return "", fmt.Errorf("error resetting %s: %s: command: %s", name, out, cmd)
	}

	// --no-commit doesn't make a commit but still requires a email and name
	// to be set. Placeholder values are used just so that git doesn't
	// error.
	h.GitArgs = append(h.GitArgs, "-c", "user.email=aur", "-c", "user.name=aur")
	cmd = h.gitCommand(filepath.Join(path, name), "merge", "--no-edit", "--no-ff", "--no-commit")
	out, err = combinedTrim(cmd)
	h.GitArgs = h.GitArgs[:len(h.GitArgs)-4]
	if err != nil {
		return "", fmt.Errorf("error merging %s: %s: command: %s", name, out, cmd)
	}

	colorWhen := "--color=always"
	if !color {
		colorWhen = "--color=never"
	}

	cmd = h.gitCommand(filepath.Join(path, name), "log", diffRef+"..HEAD@{upstream}", colorWhen)
	out1, err := combinedTrim(cmd)
	if err != nil {
		return string(out), fmt.Errorf("error diffing %s: %s: command: %s", name, out1, cmd)
	}

	cmd = h.gitCommand(filepath.Join(path, name), "diff", "--stat", "--patch", "--cached", colorWhen)
	out2, err := combinedTrim(cmd)
	if err != nil {
		return string(out), fmt.Errorf("error diffing %s: %s: command: %s", name, out2, cmd)
	}

	return out1 + "\n\n" + out2, nil
}

func (h Handle) gitPrintDiff(path string, name string) error {
	var resetRef string

	if h.gitHasRef(path, name) {
		resetRef = REF_NAME
	} else {
		resetRef = "HEAD"
	}

	cmd := h.gitCommand(filepath.Join(path, name), "reset", "--hard", resetRef)
	out, err := combinedTrim(cmd)
	if err != nil {
		return fmt.Errorf("error resetting %s: %s: command: %s", name, out, cmd)
	}

	// --no-commit doesn't make a commit but still requires a email and name
	// to be set. Placeholder values are used just so that git doesn't
	// error.
	h.GitArgs = append(h.GitArgs, "-c", "user.email=aur", "-c", "user.name=aur")
	cmd = h.gitCommand(filepath.Join(path, name), "merge", "--no-edit", "--no-ff", "--no-commit")
	out, err = combinedTrim(cmd)
	h.GitArgs = h.GitArgs[:len(h.GitArgs)-4]
	if err != nil {
		return fmt.Errorf("error merging %s: %s: command: %s", name, out, cmd)
	}

	cmd = h.gitCommand(filepath.Join(path, name), "diff", "--stat", "--patch", "--cached")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error diffing %s: command: %s", name, cmd)
	}

	return nil
}

func (h Handle) gitNeedMerge(path string, name string) bool {
	if h.gitHasRef(path, name) {
		err := h.gitCommand(filepath.Join(path, name), "merge-base", "--is-ancestor", "HEAD@{upstream}", "AUR_SEEN").Run()
		return err != nil
	} else {
		err := h.gitCommand(filepath.Join(path, name), "merge-base", "--is-ancestor", "HEAD@{upstream}", "HEAD").Run()
		return err != nil
	}
}

func (h Handle) gitRevParse(path string, args ...string) ([]string, error) {
	var outbuf bytes.Buffer

	cmd := h.gitCommand(path, "rev-parse", args...)
	cmd.Stdout = &outbuf
	err := cmd.Run()
	stdout := outbuf.String()

	if err != nil {
		return nil, fmt.Errorf("command failed: %s: %s", cmd, err.Error())
	}

	return strings.Split(stdout, "\n"), nil
}
