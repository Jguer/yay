package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
)

// HideMenus indicates if pacman's provider menus must be hidden.
var HideMenus = false

// NoConfirm indicates if user input should be skipped.
var NoConfirm = false

// Configuration stores yay's config.
type Configuration struct {
	AURURL                 string `json:"aururl" lua:"aururl"`
	AURRPCURL              string `json:"aurrpcurl" lua:"aurrpcurl"`
	BuildDir               string `json:"buildDir" lua:"build_dir"`
	Editor                 string `json:"editor" lua:"editor"`
	EditorFlags            string `json:"editorflags" lua:"editor_flags"`
	MakepkgBin             string `json:"makepkgbin" lua:"makepkg_bin"`
	MakepkgConf            string `json:"makepkgconf" lua:"makepkg_conf"`
	PacmanBin              string `json:"pacmanbin" lua:"pacman_bin"`
	PacmanConf             string `json:"pacmanconf" lua:"pacman_conf"`
	ReDownload             string `json:"redownload" lua:"redownload"`
	AnswerClean            string `json:"answerclean" lua:"-"`
	AnswerDiff             string `json:"answerdiff" lua:"-"`
	AnswerEdit             string `json:"answeredit" lua:"-"`
	AnswerUpgrade          string `json:"answerupgrade" lua:"-"`
	GitBin                 string `json:"gitbin" lua:"git_bin"`
	GpgBin                 string `json:"gpgbin" lua:"gpg_bin"`
	GpgFlags               string `json:"gpgflags" lua:"gpg_flags"`
	MFlags                 string `json:"mflags" lua:"mflags"`
	SortBy                 string `json:"sortby" lua:"sort_by"`
	SearchBy               string `json:"searchby" lua:"search_by"`
	GitFlags               string `json:"gitflags" lua:"git_flags"`
	RemoveMake             string `json:"removemake" lua:"remove_make"`
	SudoBin                string `json:"sudobin" lua:"sudo_bin"`
	SudoFlags              string `json:"sudoflags" lua:"sudo_flags"`
	Version                string `json:"version" lua:"-"`
	RequestSplitN          int    `json:"requestsplitn" lua:"request_split_n"`
	CompletionInterval     int    `json:"completionrefreshtime" lua:"completion_refresh_time"`
	MaxConcurrentDownloads int    `json:"maxconcurrentdownloads" lua:"max_concurrent_downloads"`
	BottomUp               bool   `json:"bottomup" lua:"bottom_up"`
	SudoLoop               bool   `json:"sudoloop" lua:"sudo_loop"`
	Devel                  bool   `json:"devel" lua:"devel"`
	CleanAfter             bool   `json:"cleanAfter" lua:"clean_after"`
	KeepSrc                bool   `json:"keepSrc" lua:"keep_src"`
	Provides               bool   `json:"provides" lua:"provides"`
	PGPFetch               bool   `json:"pgpfetch" lua:"pgp_fetch"`
	CleanMenu              bool   `json:"cleanmenu" lua:"clean_menu"`
	DiffMenu               bool   `json:"diffmenu" lua:"diff_menu"`
	EditMenu               bool   `json:"editmenu" lua:"edit_menu"`
	CombinedUpgrade        bool   `json:"combinedupgrade" lua:"combined_upgrade"`
	UseAsk                 bool   `json:"useask" lua:"use_ask"`
	BatchInstall           bool   `json:"batchinstall" lua:"batch_install"`
	SingleLineResults      bool   `json:"singlelineresults" lua:"single_line_results"`
	SeparateSources        bool   `json:"separatesources" lua:"separate_sources"`
	Debug                  bool   `json:"debug" lua:"debug"`
	UseRPC                 bool   `json:"rpc" lua:"rpc"`
	DoubleConfirm          bool   `json:"doubleconfirm" lua:"double_confirm"` // confirm install before and after build

	CompletionPath string `json:"-" lua:"-"`
	VCSFilePath    string `json:"-" lua:"-"`
	// ConfigPath     string `json:"-"`
	SaveConfig bool               `json:"-" lua:"-"`
	Mode       parser.TargetMode  `json:"-" lua:"-"`
	ReBuild    parser.RebuildMode `json:"rebuild" lua:"rebuild"`
}

// SaveConfig writes yay config to file.
func (c *Configuration) Save(configPath, version string) error {
	c.Version = version

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
	c.AURRPCURL = os.ExpandEnv(c.AURRPCURL)
	c.BuildDir = expandEnvOrHome(c.BuildDir)
	c.Editor = expandEnvOrHome(c.Editor)
	c.EditorFlags = os.ExpandEnv(c.EditorFlags)
	c.MakepkgBin = expandEnvOrHome(c.MakepkgBin)
	c.MakepkgConf = expandEnvOrHome(c.MakepkgConf)
	c.PacmanBin = expandEnvOrHome(c.PacmanBin)
	c.PacmanConf = expandEnvOrHome(c.PacmanConf)
	c.GpgFlags = os.ExpandEnv(c.GpgFlags)
	c.MFlags = os.ExpandEnv(c.MFlags)
	c.GitFlags = os.ExpandEnv(c.GitFlags)
	c.SortBy = os.ExpandEnv(c.SortBy)
	c.SearchBy = os.ExpandEnv(c.SearchBy)
	c.GitBin = expandEnvOrHome(c.GitBin)
	c.GpgBin = expandEnvOrHome(c.GpgBin)
	c.SudoBin = expandEnvOrHome(c.SudoBin)
	c.SudoFlags = os.ExpandEnv(c.SudoFlags)
	c.ReDownload = os.ExpandEnv(c.ReDownload)
	c.ReBuild = parser.RebuildMode(os.ExpandEnv(string(c.ReBuild)))
	c.AnswerClean = os.ExpandEnv(c.AnswerClean)
	c.AnswerDiff = os.ExpandEnv(c.AnswerDiff)
	c.AnswerEdit = os.ExpandEnv(c.AnswerEdit)
	c.AnswerUpgrade = os.ExpandEnv(c.AnswerUpgrade)
	c.RemoveMake = os.ExpandEnv(c.RemoveMake)
}

func expandEnvOrHome(path string) string {
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(os.Getenv("HOME"), path[2:])
	}

	return path
}

func (c *Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")

	if err := enc.Encode(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	return buf.String()
}

// check privilege elevator exists otherwise try to find another one.
func (c *Configuration) setPrivilegeElevator() error {
	if auth := os.Getenv("PACMAN_AUTH"); auth != "" {
		c.SudoBin = auth
		if auth != "sudo" {
			c.SudoFlags = ""
			c.SudoLoop = false
		}
	}

	for _, bin := range [...]string{c.SudoBin, "sudo"} {
		if _, err := exec.LookPath(bin); err == nil {
			c.SudoBin = bin
			return nil // wrapper or sudo command existing. Retrocompatiblity
		}
	}

	c.SudoFlags = ""
	c.SudoLoop = false

	for _, bin := range [...]string{"run0", "doas", "pkexec", "su"} {
		if _, err := exec.LookPath(bin); err == nil {
			c.SudoBin = bin
			return nil // command existing
		}
	}

	return &ErrPrivilegeElevatorNotFound{confValue: c.SudoBin}
}

func DefaultConfig(version string) *Configuration {
	return &Configuration{
		AURURL:                 "https://aur.archlinux.org",
		BuildDir:               os.ExpandEnv("$HOME/.cache/yay"),
		CleanAfter:             false,
		KeepSrc:                false,
		Editor:                 "",
		EditorFlags:            "",
		Devel:                  false,
		MakepkgBin:             "makepkg",
		MakepkgConf:            "",
		PacmanBin:              "pacman",
		PGPFetch:               true,
		PacmanConf:             "/etc/pacman.conf",
		GpgFlags:               "",
		MFlags:                 "",
		GitFlags:               "",
		BottomUp:               true,
		CompletionInterval:     7,
		MaxConcurrentDownloads: 1,
		SortBy:                 "",
		SearchBy:               "name-desc",
		SudoLoop:               false,
		GitBin:                 "git",
		GpgBin:                 "gpg",
		SudoBin:                "sudo",
		SudoFlags:              "",
		RequestSplitN:          150,
		ReDownload:             "no",
		ReBuild:                "no",
		BatchInstall:           false,
		AnswerClean:            "",
		AnswerDiff:             "",
		AnswerEdit:             "",
		AnswerUpgrade:          "",
		RemoveMake:             "ask",
		Provides:               true,
		CleanMenu:              true,
		DiffMenu:               true,
		EditMenu:               false,
		UseAsk:                 false,
		CombinedUpgrade:        true,
		SeparateSources:        true,
		Version:                version,
		Debug:                  false,
		UseRPC:                 true,
		DoubleConfirm:          true,
		Mode:                   parser.ModeAny,
	}
}

func NewConfig(logger *text.Logger, configPath, version string) (*Configuration, error) {
	newConfig := DefaultConfig(version)

	cacheHome, errCache := getCacheHome()
	if errCache != nil && logger != nil {
		logger.Errorln(errCache)
	}

	newConfig.BuildDir = cacheHome
	newConfig.CompletionPath = filepath.Join(cacheHome, completionFileName)
	newConfig.VCSFilePath = filepath.Join(cacheHome, vcsFileName)
	newConfig.load(configPath)

	if aurdest := os.Getenv("AURDEST"); aurdest != "" {
		newConfig.BuildDir = aurdest
	}

	newConfig.expandEnv()

	if newConfig.BuildDir != systemdCache {
		errBuildDir := initDir(newConfig.BuildDir)
		if errBuildDir != nil {
			return nil, errBuildDir
		}
	}

	if errPE := newConfig.setPrivilegeElevator(); errPE != nil {
		return nil, errPE
	}

	return newConfig, nil
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
