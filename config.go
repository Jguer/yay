package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	alpm "github.com/jguer/go-alpm"
)

// Verbosity settings for search
const (
	numberMenu = iota
	detailed
	minimal

	modeAUR targetMode = iota
	modeRepo
	modeAny
)

const (
	configFileName string = "yay.conf" // configFileName holds the name of the config file.
	vcsFileName    string = "vcs.json" // vcsFileName holds the name of the vcs file.
)

type targetMode int

type yayConfig struct {
	// Loaded from Config
	num     map[string]int
	value   map[string]string
	boolean map[string]bool
	// Loaded in Runtime
	configDir        string  // config directory
	cacheDir         string  // cache directory
	useColor         bool    // useColor enables/disables colored printing
	savedInfo        vcsInfo // savedInfo holds the current vcs info
	vcsFile          string  // vcsfile holds yay vcs info file path.
	shouldSaveConfig bool    // shouldSaveConfig holds whether or not the config should be saved
	searchMode       int     // searchMode controls the print method of the query
	noConfirm        bool
	mode             targetMode // Mode is used to restrict yay to AUR or repo only modes
	hideMenus        bool
}

var config yayConfig

var version = "8.1115"

// AlpmConf holds the current config values for pacman.
var pacmanConf *pacmanconf.Config

// AlpmHandle is the alpm handle used by yay.
var alpmHandle *alpm.Handle

func (y *yayConfig) defaultSettings() {
	y.noConfirm = false
	y.mode = modeAny
	y.hideMenus = false

	y.boolean = map[string]bool{
		"cleanafter":      false,
		"cleanmenu":       true,
		"combinedupgrade": false,
		"devel":           false,
		"diffmenu":        true,
		"editmenu":        false,
		"gitclone":        true,
		"pgpfetch":        true,
		"provides":        true,
		"sudoloop":        false,
		"timeupdate":      false,
		"upgrademenu":     true,
		"useask":          false,
	}

	y.num = map[string]int{
		"completioninterval": 7,
		"requestsplitn":      150,
	}

	y.value = map[string]string{
		"aururl":         "https://aur.archlinux.org",
		"answerclean":    "",
		"answerdiff":     "",
		"answeredit":     "",
		"answerupgrade":  "",
		"builddir":       "",
		"editor":         "",
		"editorflags":    "",
		"gpgcommand":     "gpg",
		"gpgflags":       "",
		"gitcommand":     "git",
		"gitflags":       "",
		"makepkgcommand": "makepkg",
		"makepkgconf":    "",
		"makepkgflags":   "",
		"pacmancommand":  "pacman",
		"pacmanconf":     "/etc/pacman.conf",
		"redownload":     "no",
		"rebuild":        "no",
		"removemake":     "ask",
		"sortby":         "votes",
		"sortmode":       "bottomup",
		"tarcommand":     "bsdtar",
	}

	if os.Getenv("XDG_CONFIG_HOME") != "" {
		y.configDir = filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "yay")
	} else if os.Getenv("HOME") != "" {
		y.configDir = filepath.Join(os.Getenv("HOME"), ".config/yay")
	} else {
		uid, _ := exec.Command("id", "-u").Output()
		y.configDir = filepath.Join("/tmp", "yay-"+(string)(uid), "config")
	}

	if os.Getenv("XDG_CACHE_HOME") != "" {
		y.cacheDir = filepath.Join(os.Getenv("XDG_CACHE_HOME"), "yay")
	} else if os.Getenv("HOME") != "" {
		y.cacheDir = filepath.Join(os.Getenv("HOME"), ".cache/yay")
	} else {
		uid, _ := exec.Command("id", "-u").Output()
		y.configDir = filepath.Join("/tmp", "yay-"+(string)(uid), "cache")
	}

	y.value["builddir"] = y.cacheDir
	y.vcsFile = filepath.Join(y.cacheDir, vcsFileName)
}

// editor returns the preferred system editor.
func editor() (string, []string) {
	switch {
	case config.value["editor"] != "":
		editor, err := exec.LookPath(config.value["editor"])
		if err != nil {
			fmt.Println(err)
		} else {
			return editor, strings.Fields(config.value["editorflags"])
		}
		fallthrough
	case os.Getenv("EDITOR") != "":
		editorArgs := strings.Fields(os.Getenv("EDITOR"))
		editor, err := exec.LookPath(editorArgs[0])
		if err != nil {
			fmt.Println(err)
		} else {
			return editor, editorArgs[1:]
		}
		fallthrough
	case os.Getenv("VISUAL") != "":
		editorArgs := strings.Fields(os.Getenv("VISUAL"))
		editor, err := exec.LookPath(editorArgs[0])
		if err != nil {
			fmt.Println(err)
		} else {
			return editor, editorArgs[1:]
		}
		fallthrough
	default:
		fmt.Println()
		fmt.Println(bold(red(arrow)), bold(cyan("$EDITOR")), bold("is not set"))
		fmt.Println(bold(red(arrow)) + bold(" Please add ") + bold(cyan("$EDITOR")) + bold(" or ") + bold(cyan("$VISUAL")) + bold(" to your environment variables."))

		for {
			fmt.Print(green(bold(arrow + " Edit PKGBUILD with: ")))
			editorInput, err := getInput("")
			if err != nil {
				fmt.Println(err)
				continue
			}

			editorArgs := strings.Fields(editorInput)

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Println(err)
				continue
			}
			return editor, editorArgs[1:]
		}
	}
}

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func continueTask(s string, cont bool) bool {
	if config.noConfirm {
		return cont
	}

	var response string
	var postFix string
	yes := "yes"
	no := "no"
	y := string([]rune(yes)[0])
	n := string([]rune(no)[0])

	if cont {
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else {
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	fmt.Print(bold(green(arrow)+" "+s), bold(postFix))

	len, err := fmt.Scanln(&response)
	if err != nil || len == 0 {
		return cont
	}

	response = strings.ToLower(response)
	return response == yes || response == y
}

func getInput(defaultValue string) (string, error) {
	if defaultValue != "" || config.noConfirm {
		fmt.Println(defaultValue)
		return defaultValue, nil
	}

	reader := bufio.NewReader(os.Stdin)

	buf, overflow, err := reader.ReadLine()
	if err != nil {
		return "", err
	}

	if overflow {
		return "", fmt.Errorf("Input too long")
	}

	return string(buf), nil
}

func (y yayConfig) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(y); err != nil {
		fmt.Println(err)
	}
	return buf.String()
}

func toUsage(usages []string) alpm.Usage {
	if len(usages) == 0 {
		return alpm.UsageAll
	}

	var ret alpm.Usage
	for _, usage := range usages {
		switch usage {
		case "Sync":
			ret |= alpm.UsageSync
		case "Search":
			ret |= alpm.UsageSearch
		case "Install":
			ret |= alpm.UsageInstall
		case "Upgrade":
			ret |= alpm.UsageUpgrade
		case "All":
			ret |= alpm.UsageAll
		}
	}

	return ret
}

func configureAlpm(conf *pacmanconf.Config) error {
	var err error

	// TODO: set SigLevel
	//sigLevel := alpm.SigPackage | alpm.SigPackageOptional | alpm.SigDatabase | alpm.SigDatabaseOptional
	//localFileSigLevel := alpm.SigUseDefault
	//remoteFileSigLevel := alpm.SigUseDefault

	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		var db *alpm.Db
		db, err = alpmHandle.RegisterSyncDb(repo.Name, 0)
		if err != nil {
			return err
		}

		db.SetServers(repo.Servers)
		db.SetUsage(toUsage(repo.Usage))

	}

	if err = alpmHandle.SetCacheDirs(pacmanConf.CacheDir...); err != nil {
		return err
	}

	// add hook directories 1-by-1 to avoid overwriting the system directory
	for _, dir := range pacmanConf.HookDir {
		if err = alpmHandle.AddHookDir(dir); err != nil {
			return err
		}
	}

	if err = alpmHandle.SetGPGDir(pacmanConf.GPGDir); err != nil {
		return err
	}

	if err = alpmHandle.SetLogFile(pacmanConf.LogFile); err != nil {
		return err
	}

	if err = alpmHandle.SetIgnorePkgs(pacmanConf.IgnorePkg...); err != nil {
		return err
	}

	if err = alpmHandle.SetIgnoreGroups(pacmanConf.IgnoreGroup...); err != nil {
		return err
	}

	if err = alpmHandle.SetArch(pacmanConf.Architecture); err != nil {
		return err
	}

	if err = alpmHandle.SetNoUpgrades(pacmanConf.NoUpgrade...); err != nil {
		return err
	}

	if alpmHandle.SetNoExtracts(pacmanConf.NoExtract...); err != nil {
		return err
	}

	/*if err = alpmHandle.SetDefaultSigLevel(sigLevel); err != nil {
		return err
	}

	if err = alpmHandle.SetLocalFileSigLevel(localFileSigLevel); err != nil {
		return err
	}

	if err = alpmHandle.SetRemoteFileSigLevel(remoteFileSigLevel); err != nil {
		return err
	}*/

	if err = alpmHandle.SetDeltaRatio(pacmanConf.UseDelta); err != nil {
		return err
	}

	if err = alpmHandle.SetUseSyslog(pacmanConf.UseSyslog); err != nil {
		return err
	}

	if err = alpmHandle.SetCheckSpace(pacmanConf.CheckSpace); err != nil {
		return err
	}

	return nil
}
