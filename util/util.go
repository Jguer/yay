package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// TarBin describes the default installation point of tar command.
const TarBin string = "/usr/bin/bsdtar"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchVerbosity determines print method used in PrintSearch
var SearchVerbosity = NumberMenu

// Verbosity settings for search
const (
	NumberMenu = iota
	Detailed
	Minimal
)

// Build controls if packages will be built from ABS.
var Build = false

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode determines top down package or down top package display
var SortMode = BottomUp

// BaseDir is the default building directory for yay
var BaseDir = "/tmp/yaytmp/"

// Describes Sorting method for numberdisplay
const (
	BottomUp = iota
	TopDown
)

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func ContinueTask(s string, def string) (cont bool) {
	if NoConfirm {
		return true
	}
	var postFix string

	if def == "nN" {
		postFix = "(Y/n)"
	} else {
		postFix = "(y/N)"
	}

	var response string
	fmt.Printf("\x1b[1;32m==> %s\x1b[1;37m %s\x1b[0m\n", s, postFix)

	fmt.Scanln(&response)
	if response == string(def[0]) || response == string(def[1]) {
		return false
	}

	return true
}

func downloadFile(path string, url string) (err error) {
	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// DownloadAndUnpack downloads url tgz and extracts to path.
func DownloadAndUnpack(url string, path string, trim bool) (err error) {
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return
	}

	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]

	tarLocation := path + fileName
	defer os.Remove(tarLocation)

	err = downloadFile(tarLocation, url)
	if err != nil {
		return
	}

	if trim {
		err = exec.Command("/bin/sh", "-c",
			TarBin+" --strip-components 2 --include='*/"+fileName[:len(fileName)-7]+"/trunk/' -xf "+tarLocation+" -C "+path).Run()
		os.Rename(path+"trunk", path+fileName[:len(fileName)-7]) // kurwa
	} else {
		err = exec.Command(TarBin, "-xf", tarLocation, "-C", path).Run()
	}
	if err != nil {
		return
	}

	return
}

// Editor returns the prefered system editor.
func Editor() string {
	if os.Getenv("EDITOR") != "" {
		return os.Getenv("EDITOR")
	} else if os.Getenv("VISUAL") != "" {
		return os.Getenv("VISUAL")
	} else {
		fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m$EDITOR\x1b[0;37;40m is not set.\x1b[0m\nPlease add $EDITOR or to your environment variables.\n")

	editorLoop:
		fmt.Printf("\x1b[32m%s\x1b[0m ", "Edit PKGBUILD with:")
		var editorInput string
		_, err := fmt.Scanln(&editorInput)
		if err != nil {
			fmt.Println(err)
			goto editorLoop
		}

		editor, err := exec.LookPath(editorInput)
		if err != nil {
			fmt.Println(err)
			goto editorLoop
		}
		return editor
	}
}
