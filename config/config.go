package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	alpm "github.com/jguer/go-alpm"
)

// Configuration stores yay's config
type Configuration struct {
	BuildDir   string
	Editor     string
	MakepkgBin string
	NoConfirm  bool
	PacmanBin  string
	PacmanConf string
	SortMode   string
	TarBin     string
}

// YayConf holds the current config values for yay.
var YayConf Configuration

// AlpmConf holds the current config values for pacman.
var AlpmConf alpm.PacmanConfig

// AlpmHandle is the alpm handle used by yay
var AlpmHandle alpm.Handle

func init() {
	configfile := os.Getenv("HOME") + "/.config/yay/config.json"

	if _, err := os.Stat(configfile); os.IsNotExist(err) {
		_ = os.MkdirAll(os.Getenv("HOME")+"/.config/yay", 0755)
	}

	file, err := os.Open(configfile)
	if err != nil {
		fmt.Println("Error reading config:", err)
		return
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&YayConf)
	if err != nil {
		fmt.Println("Error reading config:", err)
	}

	AlpmConf, err = readAlpmConfig(YayConf.PacmanConf)
	if err != nil {
		fmt.Println("Unable to read Pacman conf", err)
	}
}

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

func defaultSettings(config *Configuration) {
	config.BuildDir = "/tmp/yaytmp/"
	config.Editor = ""
	config.MakepkgBin = "/usr/bin/makepkg"
	config.NoConfirm = false
	config.PacmanBin = "/usr/bin/pacman"
	config.PacmanConf = "/etc/pacman.conf"
	config.SortMode = "BottomUp"
	config.TarBin = "/usr/bin/bsdtar"
}

// Editor returns the preferred system editor.
func Editor() string {
	switch {
	case YayConf.Editor != "":
		editor, err := exec.LookPath(YayConf.Editor)
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
func ContinueTask(s string, def string) (cont bool) {
	if YayConf.NoConfirm {
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
			YayConf.TarBin+" --strip-components 2 --include='*/"+fileName[:len(fileName)-7]+"/trunk/' -xf "+tarLocation+" -C "+path).Run()
		os.Rename(path+"trunk", path+fileName[:len(fileName)-7]) // kurwa
	} else {
		err = exec.Command(YayConf.TarBin, "-xf", tarLocation, "-C", path).Run()
	}
	if err != nil {
		return
	}

	return
}
