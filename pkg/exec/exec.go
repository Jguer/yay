package exec

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

const smallArrow = " ->"
const arrow = "==>"

// waitLock will lock yay checking the status of db.lck until it does not exist
func waitLock(pacmanConf *pacmanconf.Config) {
	if _, err := os.Stat(filepath.Join(pacmanConf.DBPath, "db.lck")); err != nil {
		return
	}

	fmt.Println(text.Bold(text.Yellow(smallArrow)), filepath.Join(pacmanConf.DBPath, "db.lck"), "is present.")

	fmt.Print(text.Bold(text.Yellow(smallArrow)), " There may be another Pacman instance running. Waiting...")

	for {
		time.Sleep(3 * time.Second)
		if _, err := os.Stat(filepath.Join(pacmanConf.DBPath, "db.lck")); err != nil {
			fmt.Println()
			return
		}
	}
}

func PassToMakepkg(bin string, flags string, makepkgConf string, dir string, args ...string) *exec.Cmd {
	mflags := strings.Fields(flags)
	args = append(args, mflags...)

	if makepkgConf != "" {
		args = append(args, "--config", makepkgConf)
	}

	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	return cmd
}

// PassToPacman generates a exec.Cmd using the provided flags and arguments
func PassToPacman(config *runtime.Configuration, pacmanConf *pacmanconf.Config, args *types.Arguments, noconfirm bool) *exec.Cmd {
	argArr := make([]string, 0)

	if args.NeedRoot(config.Mode) {
		argArr = append(argArr, "sudo")
	}

	argArr = append(argArr, config.PacmanBin)
	argArr = append(argArr, args.FormatGlobals()...)
	argArr = append(argArr, args.FormatArgs()...)
	if noconfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", config.PacmanConf)
	argArr = append(argArr, "--")
	argArr = append(argArr, args.Targets...)

	if args.NeedRoot(config.Mode) {
		waitLock(pacmanConf)
	}
	return exec.Command(argArr[0], argArr[1:]...)
}

// ShouldUseGit decides what download method to use on runtime:
// Use the config option when the destination does not already exits
// If .git exists in the destination use git
// Otherwise use a tarball
func ShouldUseGit(path string, useGit bool) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return useGit
	}

	_, err = os.Stat(filepath.Join(path, ".git"))
	return err == nil || os.IsExist(err)
}

// PassToGit runs a git command from a following dir.
func PassToGit(bin string, flags string, dir string, _args ...string) *exec.Cmd {
	gitflags := strings.Fields(flags)
	args := []string{"-C", dir}
	args = append(args, gitflags...)
	args = append(args, _args...)

	cmd := exec.Command(bin, args...)
	return cmd
}

// CaptureBin runs a bin executable with _args and returns its outputs using Capture.
// Only used for Untaring, to replace with native taring solution once binary size analysis is done
func CaptureBin(bin string, _args ...string) (string, string, error) {
	return Capture(exec.Command(bin, _args...))
}

// ShowBin runs a bin executable with _args and shows its output.
// Used for Editor
func ShowBin(bin string, _args ...string) error {
	return Show(exec.Command(bin, _args...))
}

// Capture returns the comand stdout and stderr as output.
func Capture(cmd *exec.Cmd) (string, string, error) {
	var outbuf, errbuf bytes.Buffer

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err := cmd.Run()
	stdout := strings.TrimSpace(outbuf.String())
	stderr := strings.TrimSpace(errbuf.String())

	return stdout, stderr, err
}

// Show displays the comand stdout and stderr to the user.
func Show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("")
	}
	return nil
}
