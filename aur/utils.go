package aur

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Editor gives the default system editor, uses vi in last case
var Editor = "vi"

// TarBin describes the default installation point of tar command.
const TarBin string = "/usr/bin/tar"

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// MakepkgBin describes the default installation point of makepkg command.
const MakepkgBin string = "/usr/bin/makepkg"

// SearchMode is search without numbers.
const SearchMode int = -1

// NoConfirm ignores prompts.
var NoConfirm = false

// SortMode determines top down package or down top package display
var SortMode = DownTop

// BaseDir is the default building directory for yay
var BaseDir = "/tmp/yaytmp/"

// Describes Sorting method for numberdisplay
const (
	DownTop = iota
	TopDown
)

func init() {
	if os.Getenv("EDITOR") != "" {
		Editor = os.Getenv("EDITOR")
	}
}

// getJSON handles JSON retrieval and decoding to struct
func getJSON(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}

func downloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
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

func continueTask(s string, def string) (cont bool) {
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
