package settings

import (
	"context"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
	"gopkg.in/ini.v1"
)

//go:embed yay.conf
var defaultsINI []byte

// HideMenus indicates if pacman's provider menus must be hidden.
var HideMenus = false

// NoConfirm indicates if user input should be skipped.
var NoConfirm = false

// Configuration stores yay's config.
type Configuration struct {
	AURURL                 string `json:"aururl" ini:"AurUrl"`
	AURRPCURL              string `json:"aurrpcurl" ini:"AurRpcUrl"`
	BuildDir               string `json:"buildDir" ini:"BuildDir"`
	Editor                 string `json:"editor" ini:"Editor"`
	EditorFlags            string `json:"editorflags" ini:"EditorFlags"`
	MakepkgBin             string `json:"makepkgbin" ini:"MakepkgBin"`
	MakepkgConf            string `json:"makepkgconf" ini:"MakepkgConf"`
	PacmanBin              string `json:"pacmanbin" ini:"PacmanBin"`
	PacmanConf             string `json:"pacmanconf" ini:"PacmanConf"`
	ReDownload             string `json:"redownload" ini:"ReDownload"`
	AnswerClean            string `json:"answerclean" ini:"AnswerClean"`
	AnswerDiff             string `json:"answerdiff" ini:"AnswerDiff"`
	AnswerEdit             string `json:"answeredit" ini:"AnswerEdit"`
	AnswerUpgrade          string `json:"answerupgrade" ini:"AnswerUpgrade"`
	GitBin                 string `json:"gitbin" ini:"GitBin"`
	GpgBin                 string `json:"gpgbin" ini:"GpgBin"`
	GpgFlags               string `json:"gpgflags" ini:"GpgFlags"`
	MFlags                 string `json:"mflags" ini:"MFlags"`
	SortBy                 string `json:"sortby" ini:"SortBy"`
	SearchBy               string `json:"searchby" ini:"SearchBy"`
	GitFlags               string `json:"gitflags" ini:"GitFlags"`
	RemoveMake             string `json:"removemake" ini:"RemoveMake"`
	SudoBin                string `json:"sudobin" ini:"SudoBin"`
	SudoFlags              string `json:"sudoflags" ini:"SudoFlags"`
	Version                string `json:"version" ini:"-"`
	RequestSplitN          int    `json:"requestsplitn" ini:"RequestSplitN"`
	CompletionInterval     int    `json:"completionrefreshtime" ini:"CompletionInterval"`
	MaxConcurrentDownloads int    `json:"maxconcurrentdownloads" ini:"MaxConcurrentDownloads"`
	BottomUp               bool   `json:"bottomup" ini:"BottomUp"`
	SudoLoop               bool   `json:"sudoloop" ini:"SudoLoop"`
	Devel                  bool   `json:"devel" ini:"Devel"`
	CleanAfter             bool   `json:"cleanAfter" ini:"CleanAfter"`
	KeepSrc                bool   `json:"keepSrc" ini:"KeepSrc"`
	Provides               bool   `json:"provides" ini:"Provides"`
	PGPFetch               bool   `json:"pgpfetch" ini:"PgpFetch"`
	CleanMenu              bool   `json:"cleanmenu" ini:"CleanMenu"`
	DiffMenu               bool   `json:"diffmenu" ini:"DiffMenu"`
	EditMenu               bool   `json:"editmenu" ini:"EditMenu"`
	CombinedUpgrade        bool   `json:"combinedupgrade" ini:"CombinedUpgrade"`
	UseAsk                 bool   `json:"useask" ini:"UseAsk"`
	BatchInstall           bool   `json:"batchinstall" ini:"BatchInstall"`
	SingleLineResults      bool   `json:"singlelineresults" ini:"SingleLineResults"`
	SeparateSources        bool   `json:"separatesources" ini:"SeparateSources"`
	Debug                  bool   `json:"debug" ini:"Debug"`
	UseRPC                 bool   `json:"rpc" ini:"Rpc"`
	DoubleConfirm          bool   `json:"doubleconfirm" ini:"DoubleConfirm"` // confirm install before and after build

	CompletionPath string             `json:"-" ini:"-"`
	VCSFilePath    string             `json:"-" ini:"-"`
	SaveConfig     bool               `json:"-" ini:"-"`
	Mode           parser.TargetMode  `json:"-" ini:"-"`
	ReBuild        parser.RebuildMode `json:"rebuild" ini:"ReBuild"`

	// luaEngine holds an optional Lua scripting engine when the user has an
	// init.lua config. It is kept behind an interface to avoid a pkg/settings
	// -> pkg/settings/lua import cycle. nil when no init.lua is present.
	luaEngine LuaHookRunner `json:"-" ini:"-"`
}

// LuaHookRunner is the subset of the Lua engine surface that pkg/settings
// needs at runtime. The concrete implementation lives in pkg/settings/lua.
type LuaHookRunner interface {
	CallOnPrompt(name, defaultAns string) (string, bool, error)
	CallShouldIncludeAURUpdate(candidate AURUpdateContext) (bool, bool, error)
	CallListExternalUpgrades(ctx context.Context) ([]ExternalUpgrade, error)
	CallRunExternalUpgrades(ctx context.Context, repository string, upgrades []ExternalUpgrade) (bool, error)
	CallSearchExternalPackages(ctx context.Context, terms []string) ([]ExternalSearchResult, error)
	CallInstallExternalPackages(ctx context.Context, repository string, targets []ExternalInstallTarget) (bool, error)
	HasProvider(name string) bool
	Close()
}

// ExternalUpgrade describes an upgrade candidate returned by a Lua-defined
// provider such as Homebrew or Flatpak.
type ExternalUpgrade struct {
	Name          string
	Base          string
	Repository    string
	LocalVersion  string
	RemoteVersion string
	Extra         string
}

// ExternalSearchResult describes a search result returned by a Lua-defined
// provider that can participate in mixed-source search results.
type ExternalSearchResult struct {
	Name             string
	Base             string
	Repository       string
	Version          string
	InstalledVersion string
	Description      string
	Extra            string
}

// ExternalInstallTarget identifies a package selection routed to a Lua-defined
// provider install handler.
type ExternalInstallTarget struct {
	Name       string
	Repository string
}

// AURUpdateContext describes a potential AUR upgrade candidate for Lua hooks.
// DefaultInclude is the built-in version-based decision before Lua overrides.
type AURUpdateContext struct {
	Name               string
	Base               string
	Repository         string
	LocalVersion       string
	RemoteVersion      string
	LocalBuildDate     int64
	RemoteLastModified int64
	DefaultInclude     bool
}

// SetLuaEngine attaches a Lua hook runner to the configuration. Passing nil
// detaches any existing engine without closing it.
func (c *Configuration) SetLuaEngine(e LuaHookRunner) { c.luaEngine = e }

// CloseLua releases the embedded Lua engine, if any. Safe to call multiple
// times.
func (c *Configuration) CloseLua() {
	if c.luaEngine == nil {
		return
	}
	c.luaEngine.Close()
	c.luaEngine = nil
}

// OnPrompt asks the Lua `on_prompt` hook (if any) for an answer to the named
// menu prompt. When no engine or no hook is configured, defaultAns is
// returned unchanged. Errors from Lua are written to stderr and the default
// is used as a safe fallback.
func (c *Configuration) OnPrompt(name, defaultAns string) string {
	if c == nil || c.luaEngine == nil {
		return defaultAns
	}
	got, ok, err := c.luaEngine.CallOnPrompt(name, defaultAns)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return defaultAns
	}
	if !ok {
		return defaultAns
	}
	return got
}

// ShouldIncludeAURUpdate asks the Lua `should_include_aur_update` hook (if
// any) whether an AUR package should be treated as upgradable. When no engine,
// no hook, or a hook error is present, the default version-based decision is
// returned unchanged.
func (c *Configuration) ShouldIncludeAURUpdate(candidate AURUpdateContext) bool {
	if c == nil || c.luaEngine == nil {
		return candidate.DefaultInclude
	}
	got, ok, err := c.luaEngine.CallShouldIncludeAURUpdate(candidate)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return candidate.DefaultInclude
	}
	if !ok {
		return candidate.DefaultInclude
	}
	return got
}

// ExternalUpgrades asks Lua-defined providers (if any) for additional upgrade
// candidates that should be shown alongside repo/AUR updates.
func (c *Configuration) ExternalUpgrades(ctx context.Context) ([]ExternalUpgrade, error) {
	if c == nil || c.luaEngine == nil {
		return nil, nil
	}

	upgrades, err := c.luaEngine.CallListExternalUpgrades(ctx)
	if err != nil {
		return nil, err
	}

	return upgrades, nil
}

// RunExternalUpgradeProvider asks Lua to execute the selected upgrades for a
// named provider. It is a no-op when no Lua engine or provider is configured.
func (c *Configuration) RunExternalUpgradeProvider(ctx context.Context, repository string, upgrades []ExternalUpgrade) error {
	if c == nil || c.luaEngine == nil || repository == "" || len(upgrades) == 0 {
		return nil
	}

	_, err := c.luaEngine.CallRunExternalUpgrades(ctx, repository, upgrades)
	return err
}

// SearchExternalPackages asks Lua-defined providers for additional search
// results to include in mixed-source search output.
func (c *Configuration) SearchExternalPackages(ctx context.Context, terms []string) ([]ExternalSearchResult, error) {
	if c == nil || c.luaEngine == nil {
		return nil, nil
	}

	return c.luaEngine.CallSearchExternalPackages(ctx, terms)
}

// InstallExternalPackages asks a Lua-defined provider to install the selected
// package targets. The boolean return is false when the provider is absent.
func (c *Configuration) InstallExternalPackages(ctx context.Context, repository string, targets []ExternalInstallTarget) (bool, error) {
	if c == nil || c.luaEngine == nil || repository == "" || len(targets) == 0 {
		return false, nil
	}

	return c.luaEngine.CallInstallExternalPackages(ctx, repository, targets)
}

// HasExternalProvider reports whether Lua defines a provider table for the
// given source/repository name.
func (c *Configuration) HasExternalProvider(name string) bool {
	if c == nil || c.luaEngine == nil || name == "" {
		return false
	}

	return c.luaEngine.HasProvider(name)
}

// Save writes yay config to INI file.
func (c *Configuration) Save(configPath, version string) error {
	// Use INI config path instead of JSON
	iniPath := GetINIConfigPath()
	if iniPath == "" {
		return fmt.Errorf("unable to determine config path")
	}

	return c.SaveINI(iniPath)
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
	cfg := &Configuration{
		Version: version,
		Mode:    parser.ModeAny,
	}

	// Load defaults from embedded INI
	iniCfg, err := ini.LoadSources(ini.LoadOptions{
		AllowBooleanKeys:    true,
		Insensitive:         true,
		InsensitiveSections: true,
		IgnoreInlineComment: true,
	}, defaultsINI)
	if err != nil {
		// Fallback to minimal defaults if embedded config fails
		cfg.AURURL = "https://aur.archlinux.org"
		cfg.BuildDir = os.ExpandEnv("$HOME/.cache/yay")
		return cfg
	}

	// Map the default section
	_ = iniCfg.Section("").MapTo(cfg)

	// Also map [options] section if present
	if iniCfg.HasSection("options") {
		_ = iniCfg.Section("options").MapTo(cfg)
	}

	return cfg
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

	// Experimental: if the user has an init.lua, it fully replaces INI/JSON
	// config loading. The script itself is executed later (from main, via the
	// pkg/settings/lua package); here we only branch on its presence.
	if luaPath, err := ResolveLuaConfigPath(); err == nil && luaPath != "" {
		if logger != nil {
			logger.Debugln("lua config detected, skipping INI/JSON loaders:", luaPath)
		}
		goto skipINI
	} else if err != nil && logger != nil {
		logger.Errorln(err)
	}

	// Load system-wide INI config first (silently ignored if not present)
	if err := newConfig.loadINI(SystemConfigPath); err != nil && logger != nil {
		logger.Errorln(err)
	}

	// Load user JSON config (legacy, overrides system config)
	newConfig.load(configPath)

	// Load user INI config (takes priority over JSON when both exist)
	if userINIPath := GetINIConfigPath(); userINIPath != "" {
		if err := newConfig.loadINI(userINIPath); err != nil && logger != nil {
			logger.Errorln(err)
		}
	}

skipINI:

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
