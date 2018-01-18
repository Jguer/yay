package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"

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
	Shell         string `json:"-"`
	PacmanBin     string `json:"pacmanbin"`
	PacmanConf    string `json:"pacmanconf"`
	TarBin        string `json:"tarbin"`
	RequestSplitN int    `json:"requestsplitn"`
	SearchMode    int    `json:"-"`
	SortMode      int    `json:"sortmode"`
	TimeUpdate    bool   `json:"timeupdate"`
	NoConfirm     bool   `json:"noconfirm"`
	Devel         bool   `json:"devel"`
	CleanAfter    bool   `json:"cleanAfter"`
}

const version = "2.219"

// baseURL givers the AUR default address.
const baseURL string = "https://aur.archlinux.org"

var specialDBsauce = false

var savedInfo infos

// configfile holds yay config file path.
var configFile string

// vcsfile holds yay vcs info file path.
var vcsFile string

//completion file
var completionFile string

// Updated returns if database has been updated
var updated bool

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
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	config.BuildDir = fmt.Sprintf("/tmp/yaytmp-%s/", u.Uid)
	config.CleanAfter = false
	config.Editor = ""
	config.Devel = false
	config.MakepkgBin = "/usr/bin/makepkg"
	config.NoConfirm = false
	config.PacmanBin = "/usr/bin/pacman"
	config.PacmanConf = "/etc/pacman.conf"
	config.SortMode = BottomUp
	config.TarBin = "/usr/bin/bsdtar"
	config.TimeUpdate = false
	config.RequestSplitN = 150
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

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func continueTask(s string, def string) (cont bool) {
	if config.NoConfirm {
		return true
	}
	var postFix string

	if def == "nN" {
		postFix = "[Y/n] "
	} else {
		postFix = "[y/N] "
	}

	var response string
	fmt.Printf("\x1b[1;32m==> %s\x1b[1;37m %s\x1b[0m", s, postFix)

	n, err := fmt.Scanln(&response)
	if err != nil || n == 0 {
		return true
	}

	if response == string(def[0]) || response == string(def[1]) {
		return false
	}

	return true
}
