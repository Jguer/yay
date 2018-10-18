package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
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

type targetMode int

const (
	ModeAUR targetMode = iota
	ModeRepo
	ModeAny
)

// Configuration stores yay's config.
type Configuration struct {
	AURURL             string `json:"aururl"`
	BuildDir           string `json:"buildDir"`
	Editor             string `json:"editor"`
	EditorFlags        string `json:"editorflags"`
	MakepkgBin         string `json:"makepkgbin"`
	MakepkgConf        string `json:"makepkgconf"`
	PacmanBin          string `json:"pacmanbin"`
	PacmanConf         string `json:"pacmanconf"`
	TarBin             string `json:"tarbin"`
	ReDownload         string `json:"redownload"`
	ReBuild            string `json:"rebuild"`
	AnswerClean        string `json:"answerclean"`
	AnswerDiff         string `json:"answerdiff"`
	AnswerEdit         string `json:"answeredit"`
	AnswerUpgrade      string `json:"answerupgrade"`
	GitBin             string `json:"gitbin"`
	GpgBin             string `json:"gpgbin"`
	GpgFlags           string `json:"gpgflags"`
	MFlags             string `json:"mflags"`
	SortBy             string `json:"sortby"`
	GitFlags           string `json:"gitflags"`
	RemoveMake         string `json:"removemake"`
	RequestSplitN      int    `json:"requestsplitn"`
	SearchMode         int    `json:"-"`
	SortMode           int    `json:"sortmode"`
	CompletionInterval int    `json:"completionrefreshtime"`
	SudoLoop           bool   `json:"sudoloop"`
	TimeUpdate         bool   `json:"timeupdate"`
	NoConfirm          bool   `json:"-"`
	Devel              bool   `json:"devel"`
	CleanAfter         bool   `json:"cleanAfter"`
	GitClone           bool   `json:"gitclone"`
	Provides           bool   `json:"provides"`
	PGPFetch           bool   `json:"pgpfetch"`
	UpgradeMenu        bool   `json:"upgrademenu"`
	CleanMenu          bool   `json:"cleanmenu"`
	DiffMenu           bool   `json:"diffmenu"`
	EditMenu           bool   `json:"editmenu"`
	CombinedUpgrade    bool   `json:"combinedupgrade"`
	UseAsk             bool   `json:"useask"`
}

var version = "8.1173"

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

// useColor enables/disables colored printing
var useColor bool

// configHome handles config directory home
var configHome string

// cacheHome handles cache home
var cacheHome string

// savedInfo holds the current vcs info
var savedInfo vcsInfo

// configfile holds yay config file path.
var configFile string

// vcsfile holds yay vcs info file path.
var vcsFile string

// shouldSaveConfig holds whether or not the config should be saved
var shouldSaveConfig bool

// YayConf holds the current config values for yay.
var config Configuration

// AlpmConf holds the current config values for pacman.
var pacmanConf *pacmanconf.Config

// AlpmHandle is the alpm handle used by yay.
var alpmHandle *alpm.Handle

// Mode is used to restrict yay to AUR or repo only modes
var mode = ModeAny

var hideMenus = false

// SaveConfig writes yay config to file.
func (config *Configuration) saveConfig() error {
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

func (config *Configuration) defaultSettings() {
	buildDir := "$HOME/.cache/yay"
	if os.Getenv("XDG_CACHE_HOME") != "" {
		buildDir = "$XDG_CACHE_HOME/yay"
	}

	config.AURURL = "https://aur.archlinux.org"
	config.BuildDir = buildDir
	config.CleanAfter = false
	config.Editor = ""
	config.EditorFlags = ""
	config.Devel = false
	config.MakepkgBin = "makepkg"
	config.MakepkgConf = ""
	config.NoConfirm = false
	config.PacmanBin = "pacman"
	config.PGPFetch = true
	config.PacmanConf = "/etc/pacman.conf"
	config.GpgFlags = ""
	config.MFlags = ""
	config.GitFlags = ""
	config.SortMode = BottomUp
	config.CompletionInterval = 7
	config.SortBy = "votes"
	config.SudoLoop = false
	config.TarBin = "bsdtar"
	config.GitBin = "git"
	config.GpgBin = "gpg"
	config.TimeUpdate = false
	config.RequestSplitN = 150
	config.ReDownload = "no"
	config.ReBuild = "no"
	config.AnswerClean = ""
	config.AnswerDiff = ""
	config.AnswerEdit = ""
	config.AnswerUpgrade = ""
	config.RemoveMake = "ask"
	config.GitClone = true
	config.Provides = true
	config.UpgradeMenu = true
	config.CleanMenu = true
	config.DiffMenu = true
	config.EditMenu = false
	config.UseAsk = false
	config.CombinedUpgrade = false
}

func (config *Configuration) expandEnv() {
	config.AURURL = os.ExpandEnv(config.AURURL)
	config.BuildDir = os.ExpandEnv(config.BuildDir)
	config.Editor = os.ExpandEnv(config.Editor)
	config.EditorFlags = os.ExpandEnv(config.EditorFlags)
	config.MakepkgBin = os.ExpandEnv(config.MakepkgBin)
	config.MakepkgConf = os.ExpandEnv(config.MakepkgConf)
	config.PacmanBin = os.ExpandEnv(config.PacmanBin)
	config.PacmanConf = os.ExpandEnv(config.PacmanConf)
	config.GpgFlags = os.ExpandEnv(config.GpgFlags)
	config.MFlags = os.ExpandEnv(config.MFlags)
	config.GitFlags = os.ExpandEnv(config.GitFlags)
	config.SortBy = os.ExpandEnv(config.SortBy)
	config.TarBin = os.ExpandEnv(config.TarBin)
	config.GitBin = os.ExpandEnv(config.GitBin)
	config.GpgBin = os.ExpandEnv(config.GpgBin)
	config.ReDownload = os.ExpandEnv(config.ReDownload)
	config.ReBuild = os.ExpandEnv(config.ReBuild)
	config.AnswerClean = os.ExpandEnv(config.AnswerClean)
	config.AnswerDiff = os.ExpandEnv(config.AnswerDiff)
	config.AnswerEdit = os.ExpandEnv(config.AnswerEdit)
	config.AnswerUpgrade = os.ExpandEnv(config.AnswerUpgrade)
	config.RemoveMake = os.ExpandEnv(config.RemoveMake)
}

// Editor returns the preferred system editor.
func editor() (string, []string) {
	switch {
	case config.Editor != "":
		editor, err := exec.LookPath(config.Editor)
		if err != nil {
			fmt.Println(err)
		} else {
			return editor, strings.Fields(config.EditorFlags)
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
	if config.NoConfirm {
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
	if defaultValue != "" || config.NoConfirm {
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

func (config Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(config); err != nil {
		fmt.Println(err)
	}
	return buf.String()
}

func toUsage(usages []string) alpm.Usage {
	if len(usages) == 0 {
		return alpm.UsageAll
	}

	var ret alpm.Usage = 0
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
		db, err := alpmHandle.RegisterSyncDb(repo.Name, 0)
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
