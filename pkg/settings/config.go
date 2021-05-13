package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

const (
	// Describes Sorting method for numberdisplay
	BottomUp = iota
	TopDown
)

// HideMenus indicates if pacman's provider menus must be hidden
var HideMenus = false

// NoConfirm indicates if user input should be skipped
var NoConfirm = false

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
func (c *Configuration) Save(configPath string) error {
	marshalledinfo, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		return err
	}

	// https://github.com/Jguer/yay/issues/1325
	marshalledinfo = append(marshalledinfo, '\n')
	// https://github.com/Jguer/yay/issues/1399
	if _, err = os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) && err != nil {
		if mkErr := os.MkdirAll(filepath.Dir(configPath), 0o755); mkErr != nil {
			return mkErr
		}
	}
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

func (c *Configuration) expandEnv() {
	c.AURURL = os.ExpandEnv(c.AURURL)
	c.ABSDir = os.ExpandEnv(c.ABSDir)
	c.BuildDir = os.ExpandEnv(c.BuildDir)
	c.Editor = os.ExpandEnv(c.Editor)
	c.EditorFlags = os.ExpandEnv(c.EditorFlags)
	c.MakepkgBin = os.ExpandEnv(c.MakepkgBin)
	c.MakepkgConf = os.ExpandEnv(c.MakepkgConf)
	c.PacmanBin = os.ExpandEnv(c.PacmanBin)
	c.PacmanConf = os.ExpandEnv(c.PacmanConf)
	c.GpgFlags = os.ExpandEnv(c.GpgFlags)
	c.MFlags = os.ExpandEnv(c.MFlags)
	c.GitFlags = os.ExpandEnv(c.GitFlags)
	c.SortBy = os.ExpandEnv(c.SortBy)
	c.SearchBy = os.ExpandEnv(c.SearchBy)
	c.GitBin = os.ExpandEnv(c.GitBin)
	c.GpgBin = os.ExpandEnv(c.GpgBin)
	c.SudoBin = os.ExpandEnv(c.SudoBin)
	c.SudoFlags = os.ExpandEnv(c.SudoFlags)
	c.ReDownload = os.ExpandEnv(c.ReDownload)
	c.ReBuild = os.ExpandEnv(c.ReBuild)
	c.AnswerClean = os.ExpandEnv(c.AnswerClean)
	c.AnswerDiff = os.ExpandEnv(c.AnswerDiff)
	c.AnswerEdit = os.ExpandEnv(c.AnswerEdit)
	c.AnswerUpgrade = os.ExpandEnv(c.AnswerUpgrade)
	c.RemoveMake = os.ExpandEnv(c.RemoveMake)
}

func (c *Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(c); err != nil {
		text.Errorln(err)
	}
	return buf.String()
}

func DefaultConfig() *Configuration {
	return &Configuration{
		AURURL:             "https://aur.archlinux.org",
		BuildDir:           os.ExpandEnv("$HOME/.cache/yay"),
		ABSDir:             os.ExpandEnv("$HOME/.cache/yay/abs"),
		CleanAfter:         false,
		Editor:             "",
		EditorFlags:        "",
		Devel:              false,
		MakepkgBin:         "makepkg",
		MakepkgConf:        "",
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
}

func NewConfig() (*Configuration, error) {
	newConfig := DefaultConfig()

	cacheHome := getCacheHome()
	newConfig.BuildDir = cacheHome

	configPath := getConfigPath()
	newConfig.load(configPath)

	if aurdest := os.Getenv("AURDEST"); aurdest != "" {
		newConfig.BuildDir = aurdest
	}

	newConfig.expandEnv()

	newConfig.Runtime = &Runtime{
		ConfigPath:     configPath,
		Mode:           ModeAny,
		SaveConfig:     false,
		CompletionPath: filepath.Join(cacheHome, completionFileName),
		CmdRunner:      &exe.OSRunner{},
		CmdBuilder: &exe.CmdBuilder{
			GitBin:          newConfig.GitBin,
			GitFlags:        strings.Fields(newConfig.GitFlags),
			MakepkgFlags:    strings.Fields(newConfig.MFlags),
			MakepkgConfPath: newConfig.MakepkgConf,
			MakepkgBin:      newConfig.MakepkgBin,
		},
		PacmanConf: nil,
		VCSStore:   nil,
		HTTPClient: &http.Client{},
	}

	newConfig.Runtime.VCSStore = vcs.NewInfoStore(filepath.Join(cacheHome, vcsFileName),
		newConfig.Runtime.CmdRunner, newConfig.Runtime.CmdBuilder)

	if err := initDir(newConfig.BuildDir); err != nil {
		return nil, err
	}

	err := newConfig.Runtime.VCSStore.Load()

	return newConfig, err
}

func (c *Configuration) load(configPath string) {
	cfile, err := os.Open(configPath)
	if !os.IsNotExist(err) && err != nil {
		fmt.Fprintln(os.Stderr,
			gotext.Get("failed to open config file '%s': %s", configPath, err))
		return
	}

	defer cfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(cfile)
		if err = decoder.Decode(c); err != nil {
			fmt.Fprintln(os.Stderr,
				gotext.Get("failed to read config file '%s': %s", configPath, err))
		}
	}
}
