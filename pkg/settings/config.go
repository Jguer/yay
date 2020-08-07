package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const (
	// Describes Sorting method for numberdisplay
	BottomUp = iota
	TopDown
)

// HideMenus indicates if pacman's provider menus must be hidden
var HideMenus = false

// Configuration stores yay's config.
type Configuration struct {
	AURURL             string   `json:"aururl"`
	BuildDir           string   `json:"buildDir"`
	ABSDir             string   `json:"absdir"`
	Editor             string   `json:"editor"`
	EditorFlags        string   `json:"editorflags"`
	MakepkgBin         string   `json:"makepkgbin"`
	MakepkgConf        string   `json:"makepkgconf"`
	PacmanBin          string   `json:"pacmanbin"`
	PacmanConf         string   `json:"pacmanconf"`
	ReDownload         string   `json:"redownload"`
	ReBuild            string   `json:"rebuild"`
	AnswerClean        string   `json:"answerclean"`
	AnswerDiff         string   `json:"answerdiff"`
	AnswerEdit         string   `json:"answeredit"`
	AnswerUpgrade      string   `json:"answerupgrade"`
	GitBin             string   `json:"gitbin"`
	GpgBin             string   `json:"gpgbin"`
	GpgFlags           string   `json:"gpgflags"`
	MFlags             string   `json:"mflags"`
	SortBy             string   `json:"sortby"`
	SearchBy           string   `json:"searchby"`
	GitFlags           string   `json:"gitflags"`
	RemoveMake         string   `json:"removemake"`
	SudoBin            string   `json:"sudobin"`
	SudoFlags          string   `json:"sudoflags"`
	RequestSplitN      int      `json:"requestsplitn"`
	SearchMode         int      `json:"-"`
	SortMode           int      `json:"sortmode"`
	CompletionInterval int      `json:"completionrefreshtime"`
	SudoLoop           bool     `json:"sudoloop"`
	TimeUpdate         bool     `json:"timeupdate"`
	NoConfirm          bool     `json:"-"`
	Devel              bool     `json:"devel"`
	CleanAfter         bool     `json:"cleanAfter"`
	Provides           bool     `json:"provides"`
	PGPFetch           bool     `json:"pgpfetch"`
	UpgradeMenu        bool     `json:"upgrademenu"`
	CleanMenu          bool     `json:"cleanmenu"`
	DiffMenu           bool     `json:"diffmenu"`
	EditMenu           bool     `json:"editmenu"`
	CombinedUpgrade    bool     `json:"combinedupgrade"`
	UseAsk             bool     `json:"useask"`
	BatchInstall       bool     `json:"batchinstall"`
	Runtime            *Runtime `json:"-"`
}

// SaveConfig writes yay config to file.
func (config *Configuration) SaveConfig(configPath string) error {
	marshalledinfo, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}
	// https://github.com/Jguer/yay/issues/1325
	marshalledinfo = append(marshalledinfo, '\n')
	in, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err = in.Write(marshalledinfo); err != nil {
		return err
	}
	return in.Sync()
}

func (config *Configuration) ExpandEnv() {
	config.AURURL = os.ExpandEnv(config.AURURL)
	config.ABSDir = os.ExpandEnv(config.ABSDir)
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
	config.SearchBy = os.ExpandEnv(config.SearchBy)
	config.GitBin = os.ExpandEnv(config.GitBin)
	config.GpgBin = os.ExpandEnv(config.GpgBin)
	config.SudoBin = os.ExpandEnv(config.SudoBin)
	config.SudoFlags = os.ExpandEnv(config.SudoFlags)
	config.ReDownload = os.ExpandEnv(config.ReDownload)
	config.ReBuild = os.ExpandEnv(config.ReBuild)
	config.AnswerClean = os.ExpandEnv(config.AnswerClean)
	config.AnswerDiff = os.ExpandEnv(config.AnswerDiff)
	config.AnswerEdit = os.ExpandEnv(config.AnswerEdit)
	config.AnswerUpgrade = os.ExpandEnv(config.AnswerUpgrade)
	config.RemoveMake = os.ExpandEnv(config.RemoveMake)
}

func (config *Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(config); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return buf.String()
}

func MakeConfig() *Configuration {
	newConfig := &Configuration{
		AURURL:             "https://aur.archlinux.org",
		BuildDir:           "$HOME/.cache/yay",
		ABSDir:             "$HOME/.cache/yay/abs",
		CleanAfter:         false,
		Editor:             "",
		EditorFlags:        "",
		Devel:              false,
		MakepkgBin:         "makepkg",
		MakepkgConf:        "",
		NoConfirm:          false,
		PacmanBin:          "pacman",
		PGPFetch:           true,
		PacmanConf:         "/etc/pacman.conf",
		GpgFlags:           "",
		MFlags:             "",
		GitFlags:           "",
		SortMode:           BottomUp,
		CompletionInterval: 7,
		SortBy:             "votes",
		SearchBy:           "name-desc",
		SudoLoop:           false,
		GitBin:             "git",
		GpgBin:             "gpg",
		SudoBin:            "sudo",
		SudoFlags:          "",
		TimeUpdate:         false,
		RequestSplitN:      150,
		ReDownload:         "no",
		ReBuild:            "no",
		BatchInstall:       false,
		AnswerClean:        "",
		AnswerDiff:         "",
		AnswerEdit:         "",
		AnswerUpgrade:      "",
		RemoveMake:         "ask",
		Provides:           true,
		UpgradeMenu:        true,
		CleanMenu:          true,
		DiffMenu:           true,
		EditMenu:           false,
		UseAsk:             false,
		CombinedUpgrade:    false,
	}

	if os.Getenv("XDG_CACHE_HOME") != "" {
		newConfig.BuildDir = "$XDG_CACHE_HOME/yay"
	}

	return newConfig
}
