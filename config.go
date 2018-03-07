package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	alpm "github.com/jguer/go-alpm"
)

// Verbosity settings for search
const (
	NumberMenu = iota
	Detailed
	Minimal
)

// Describes Sorting method for numberdisplay
const (
	BottomUp = iota
	TopDown
)

// Configuration stores yay's config.
type Configuration struct {
	BuildDir      string `json:"buildDir"`
	Editor        string `json:"editor"`
	MakepkgBin    string `json:"makepkgbin"`
	PacmanBin     string `json:"pacmanbin"`
	PacmanConf    string `json:"pacmanconf"`
	TarBin        string `json:"tarbin"`
	ReDownload    string `json:"redownload"`
	RequestSplitN int    `json:"requestsplitn"`
	SearchMode    int    `json:"-"`
	SortMode      int    `json:"sortmode"`
	SudoLoop      bool   `json:"sudoloop"`
	TimeUpdate    bool   `json:"timeupdate"`
	NoConfirm     bool   `json:"-"`
	Devel         bool   `json:"devel"`
	CleanAfter    bool   `json:"cleanAfter"`
}

var version = "3.373"

// baseURL givers the AUR default address.
const baseURL string = "https://aur.archlinux.org"

var savedInfo infos

// configfile holds yay config file path.
var configFile string

// vcsfile holds yay vcs info file path.
var vcsFile string

//completion file
var completionFile string

// changedConfig holds whether or not the config has changed
var changedConfig bool

// YayConf holds the current config values for yay.
var config Configuration

// AlpmConf holds the current config values for pacman.
var alpmConf alpm.PacmanConfig

// AlpmHandle is the alpm handle used by yay.
var alpmHandle *alpm.Handle

func readAlpmConfig(pacmanconf string) (conf alpm.PacmanConfig, err error) {
	file, err := os.Open(pacmanconf)
	if err != nil {
		return
	}
	conf, err = alpm.ParseConfig(file)
	if err != nil {
		return
	}
	return
}

// SaveConfig writes yay config to file.
func (config *Configuration) saveConfig() error {
	config.NoConfirm = false
	marshalledinfo, _ := json.MarshalIndent(config, "", "\t")
	in, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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

func defaultSettings(config *Configuration) {
	config.BuildDir = fmt.Sprintf("%s/.cache/yay/", os.Getenv("HOME"))
	config.CleanAfter = false
	config.Editor = ""
	config.Devel = false
	config.MakepkgBin = "/usr/bin/makepkg"
	config.NoConfirm = false
	config.PacmanBin = "/usr/bin/pacman"
	config.PacmanConf = "/etc/pacman.conf"
	config.SortMode = BottomUp
	config.SudoLoop = false
	config.TarBin = "/usr/bin/bsdtar"
	config.TimeUpdate = false
	config.RequestSplitN = 150
	config.ReDownload = "no"
}

// Editor returns the preferred system editor.
func editor() string {
	switch {
	case config.Editor != "":
		editor, err := exec.LookPath(config.Editor)
		if err != nil {
			fmt.Println(err)
		} else {
			return editor
		}
		fallthrough
	case os.Getenv("EDITOR") != "":
		editor, err := exec.LookPath(os.Getenv("EDITOR"))
		if err != nil {
			fmt.Println(err)
		} else {
			return editor
		}
		fallthrough
	case os.Getenv("VISUAL") != "":
		editor, err := exec.LookPath(os.Getenv("VISUAL"))
		if err != nil {
			fmt.Println(err)
		} else {
			return editor
		}
		fallthrough
	default:
		fmt.Println(bold(red("Warning:")),
			bold(magenta("$EDITOR")), "is not set")
		fmt.Println("Please add $EDITOR or to your environment variables.")

	editorLoop:
		fmt.Print(green("Edit PKGBUILD with:"))
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

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func continueTask(s string, def string) (cont bool) {
	if config.NoConfirm {
		return true
	}
	var postFix string

	if def == "nN" {
		postFix = " [Y/n] "
	} else {
		postFix = " [y/N] "
	}

	var response string
	fmt.Print(bold(green(arrow+" "+s+" ")), bold(postFix))

	n, err := fmt.Scanln(&response)
	if err != nil || n == 0 {
		return true
	}

	if response == string(def[0]) || response == string(def[1]) {
		return false
	}

	return true
}

func (config Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(config); err != nil {
		fmt.Println(err)
	}
	return buf.String()
}
