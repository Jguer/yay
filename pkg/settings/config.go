package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/vcs"

	"github.com/Jguer/aur/metadata"
	"github.com/Jguer/aur/rpc"
	"github.com/Jguer/votar/pkg/vote"
	"github.com/leonelquinteros/gotext"
)

// HideMenus indicates if pacman's provider menus must be hidden.
var HideMenus = false

// NoConfirm indicates if user input should be skipped.
var NoConfirm = false

// Configuration stores yay's config.
type Configuration struct {
	Runtime                *Runtime `json:"-"`
	AURURL                 string   `json:"aururl"`
	AURRPCURL              string   `json:"aurrpcurl"`
	BuildDir               string   `json:"buildDir"`
	Editor                 string   `json:"editor"`
	EditorFlags            string   `json:"editorflags"`
	MakepkgBin             string   `json:"makepkgbin"`
	MakepkgConf            string   `json:"makepkgconf"`
	PacmanBin              string   `json:"pacmanbin"`
	PacmanConf             string   `json:"pacmanconf"`
	ReDownload             string   `json:"redownload"`
	ReBuild                string   `json:"rebuild"`
	AnswerClean            string   `json:"answerclean"`
	AnswerDiff             string   `json:"answerdiff"`
	AnswerEdit             string   `json:"answeredit"`
	AnswerUpgrade          string   `json:"answerupgrade"`
	GitBin                 string   `json:"gitbin"`
	GpgBin                 string   `json:"gpgbin"`
	GpgFlags               string   `json:"gpgflags"`
	MFlags                 string   `json:"mflags"`
	SortBy                 string   `json:"sortby"`
	SearchBy               string   `json:"searchby"`
	GitFlags               string   `json:"gitflags"`
	RemoveMake             string   `json:"removemake"`
	SudoBin                string   `json:"sudobin"`
	SudoFlags              string   `json:"sudoflags"`
	Version                string   `json:"version"`
	RequestSplitN          int      `json:"requestsplitn"`
	CompletionInterval     int      `json:"completionrefreshtime"`
	MaxConcurrentDownloads int      `json:"maxconcurrentdownloads"`
	BottomUp               bool     `json:"bottomup"`
	SudoLoop               bool     `json:"sudoloop"`
	TimeUpdate             bool     `json:"timeupdate"`
	Devel                  bool     `json:"devel"`
	CleanAfter             bool     `json:"cleanAfter"`
	Provides               bool     `json:"provides"`
	PGPFetch               bool     `json:"pgpfetch"`
	UpgradeMenu            bool     `json:"upgrademenu"`
	CleanMenu              bool     `json:"cleanmenu"`
	DiffMenu               bool     `json:"diffmenu"`
	EditMenu               bool     `json:"editmenu"`
	CombinedUpgrade        bool     `json:"combinedupgrade"`
	UseAsk                 bool     `json:"useask"`
	BatchInstall           bool     `json:"batchinstall"`
	SingleLineResults      bool     `json:"singlelineresults"`
	SeparateSources        bool     `json:"separatesources"`
	NewInstallEngine       bool     `json:"newinstallengine"`
	Debug                  bool     `json:"debug"`
	UseRPC                 bool     `json:"rpc"`
}

// SaveConfig writes yay config to file.
func (c *Configuration) Save(configPath string) error {
	c.Version = c.Runtime.Version

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
	c.ReBuild = os.ExpandEnv(c.ReBuild)
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

	for _, bin := range [...]string{"doas", "pkexec", "su"} {
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
		MaxConcurrentDownloads: 0,
		SortBy:                 "votes",
		SearchBy:               "name-desc",
		SudoLoop:               false,
		GitBin:                 "git",
		GpgBin:                 "gpg",
		SudoBin:                "sudo",
		SudoFlags:              "",
		TimeUpdate:             false,
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
		UpgradeMenu:            true,
		CleanMenu:              true,
		DiffMenu:               true,
		EditMenu:               false,
		UseAsk:                 false,
		CombinedUpgrade:        false,
		SeparateSources:        true,
		NewInstallEngine:       true,
		Version:                version,
		Debug:                  false,
		UseRPC:                 true,
	}
}

func NewConfig(version string) (*Configuration, error) {
	newConfig := DefaultConfig(version)

	cacheHome, errCache := getCacheHome()
	if errCache != nil {
		text.Errorln(errCache)
	}

	newConfig.BuildDir = cacheHome

	configPath := getConfigPath()
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

	userAgent := fmt.Sprintf("Yay/%s", version)

	voteClient, errVote := vote.NewClient(vote.WithUserAgent(userAgent))
	if errVote != nil {
		return nil, errVote
	}

	voteClient.SetCredentials(
		os.Getenv("AUR_USERNAME"),
		os.Getenv("AUR_PASSWORD"))

	newConfig.Runtime = &Runtime{
		ConfigPath:     configPath,
		Version:        version,
		Mode:           parser.ModeAny,
		SaveConfig:     false,
		CompletionPath: filepath.Join(cacheHome, completionFileName),
		CmdBuilder:     newConfig.CmdBuilder(nil),
		PacmanConf:     nil,
		VCSStore:       nil,
		HTTPClient:     &http.Client{},
		AURClient:      nil,
		VoteClient:     voteClient,
		QueryBuilder:   nil,
		Logger:         text.NewLogger(os.Stdout, os.Stdin, newConfig.Debug, "runtime"),
	}

	var errAURCache error

	userAgentFn := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("User-Agent", userAgent)
		return nil
	}

	newConfig.Runtime.AURCache, errAURCache = metadata.New(
		metadata.WithHTTPClient(newConfig.Runtime.HTTPClient),
		metadata.WithCacheFilePath(filepath.Join(newConfig.BuildDir, "aur.json")),
		metadata.WithRequestEditorFn(userAgentFn),
		metadata.WithBaseURL(newConfig.AURURL),
		metadata.WithDebugLogger(newConfig.Runtime.Logger.Child("aur").Debugln),
	)
	if errAURCache != nil {
		return nil, fmt.Errorf(gotext.Get("failed to retrieve aur Cache")+": %w", errAURCache)
	}

	var errAUR error
	newConfig.Runtime.AURClient, errAUR = rpc.NewClient(
		rpc.WithHTTPClient(newConfig.Runtime.HTTPClient),
		rpc.WithRequestEditorFn(userAgentFn),
		rpc.WithLogFn(newConfig.Runtime.Logger.Child("rpc").Debugln))
	if errAUR != nil {
		return nil, errAUR
	}

	if newConfig.UseRPC {
		newConfig.Runtime.AURCache = newConfig.Runtime.AURClient
	}

	newConfig.Runtime.VCSStore = vcs.NewInfoStore(
		filepath.Join(cacheHome, vcsFileName), newConfig.Runtime.CmdBuilder)

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

func (c *Configuration) CmdBuilder(runner exe.Runner) exe.ICmdBuilder {
	if runner == nil {
		runner = &exe.OSRunner{}
	}

	return &exe.CmdBuilder{
		GitBin:           c.GitBin,
		GitFlags:         strings.Fields(c.GitFlags),
		GPGBin:           c.GpgBin,
		GPGFlags:         strings.Fields(c.GpgFlags),
		MakepkgFlags:     strings.Fields(c.MFlags),
		MakepkgConfPath:  c.MakepkgConf,
		MakepkgBin:       c.MakepkgBin,
		SudoBin:          c.SudoBin,
		SudoFlags:        strings.Fields(c.SudoFlags),
		SudoLoopEnabled:  c.SudoLoop,
		PacmanBin:        c.PacmanBin,
		PacmanConfigPath: c.PacmanConf,
		PacmanDBPath:     "",
		Runner:           runner,
	}
}
