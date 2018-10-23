package settings

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Morganamilo/go-pacmanconf"
	"github.com/Morganamilo/go-pacmanconf/ini"
	"github.com/leonelquinteros/gotext"
)

const (
	Version     = "v"
	Database    = "D"
	Files       = "F"
	Query       = "Q"
	Remove      = "R"
	Sync        = "S"
	DepTest     = "T"
	Upgrade     = "U"
	Yay         = "Y"
	Show        = "P"
	GetPkgbuild = "G"
)

const (
	BottomUp = "bottomup"
	TopDown  = "topdown"
)

// configFileName holds the name of the config file.
const configFileName string = "yay.conf"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

const completionFileName string = "completion.cache"

const (
	ModeAny  = "any"
	ModeAUR  = "aur"
	ModeRepo = "repo"
)

type YayConfig struct {
	AURURL   string `section:"options" value:"required"`
	BuildDir string `section:"options" value:"required"`
	ABSDir   string `section:"options" value:"required"`

	Redownload string `section:"options" value:"optional" values:"yes,no,all" no:""`
	Rebuild    string `section:"options" value:"optional" values:"yes,no,all,tree" no:""`
	RemoveMake string `section:"options" value:"optional" values:"yes,no,ask" no:""`
	SortBy     string `section:"options" value:"required" values:"votes,popularity,name,base,submitted,modified,id,baseid"`
	SearchBy   string `section:"options" value:"required" values:"name,name-desc.maintainer,depends,checkdepends,makedepends,optdepends"`
	SortMode   string `section:"options" value:"required" values:"topdown,bottomup" alias:""`
	Mode       string `section:"options" value:"required" values:"aur,repo" alias:""`

	RequestSplitN      int `section:"options" value:"required"`
	CompletionInterval int `section:"options" value:"required"`

	SudoLoop        bool `section:"options" value:"none"`
	TimeUpdate      bool `section:"options" value:"none"`
	Devel           bool `section:"options" value:"none"`
	CleanAfter      bool `section:"options" value:"none"`
	GitClone        bool `section:"options" value:"none"`
	Provides        bool `section:"options" value:"none"`
	PGPFetch        bool `section:"options" value:"none"`
	CombinedUpgrade bool `section:"options" value:"none"`
	UseAsk          bool `section:"options" value:"none"`
	BatchInstall    bool `section:"options" value:"none"`

	Editor     string `section:"bin" long:"editor" value:"required"`
	MakepkgBin string `section:"bin" long:"makepkg" value:"required"`
	PacmanBin  string `section:"bin" long:"pacman" value:"required"`
	TarBin     string `section:"bin" long:"tar" value:"required"`
	GitBin     string `section:"bin" long:"git" value:"required"`
	GpgBin     string `section:"bin" long:"gpg" value:"required"`
	SudoBin    string `section:"bin" long:"sudo" value:"required"`

	EditorFlags []string `section:"bin" value:"required" split:"" no:""`
	MFlags      []string `section:"bin" value:"required" split:"" no:""`
	GitFlags    []string `section:"bin" value:"required" split:"" no:""`
	GpgFlags    []string `section:"bin" value:"required" split:"" no:""`
	SudoFlags   []string `section:"bin" value:"required" split:"" no:""`

	MakepkgConf string `section:"bin" value:"required"`
	PacmanConf  string `section:"bin" value:"required"`

	UpgradeMenu bool `section:"menu" long:"upgrade"`
	CleanMenu   bool `section:"menu" long:"clean"`
	DiffMenu    bool `section:"menu" long:"diff"`
	EditMenu    bool `section:"menu" long:"edit"`

	AnswerUpgrade string `section:"menu" long:"upgrade" no:""`
	AnswerClean   string `section:"menu" long:"clean" no:""`
	AnswerDiff    string `section:"menu" long:"diff" no:""`
	AnswerEdit    string `section:"menu" long:"edit" no:""`

	NumUpgrades bool `flag:"" short:"n"`
	News        int  `flag:"" short:"w"`
	Complete    int  `flag:"" short:"c"`
	Stats       bool `flag:"" short:"s"`
	Gendb       bool `flag:""`
	Force       bool `flag:"" short:"f"`

	EndOfArgs bool `flag:"" long:"--" value:"none"`
	Stdin     bool `flag:"" long:"-"`

	//pacman globals
	DbPath                 string   `pflag:"" short:"b" value:"required" global:""`
	Root                   string   `pflag:"" short:"r" value:"required" global:""`
	Verbose                bool     `pflag:"" short:"v" value:"none" global:""`
	Ask                    string   `pflag:"" value:"required" global:""`
	Arch                   string   `pflag:"" value:"required" global:""`
	CacheDir               []string `pflag:"" value:"required" global:""`
	Color                  string   `pflag:"" value:"optional" values:"always,never,optional" global:""`
	PacmanConfig           string   `name:"config" section:"options" value:"required" global:""`
	Debug                  string   `pflag:"" value:"none" global:""`
	GpgDir                 string   `pflag:"" value:"required" global:""`
	HookDir                []string `pflag:"" value:"required" global:""`
	LogFile                string   `pflag:"" value:"required" global:""`
	DisableDownloadTimeout bool     `pflag:"" long:"disable-download-timeout" value:"none" global:""`
	Sysroot                string   `pflag:"" value:"required" global:""`
	Ignore                 []string `pflag:"" value:"required" split:"," global:""`
	IgnoreGroup            []string `pflag:"" value:"required" split:"," global:""`
	Op                     string

	Nodeps          int      `pflag:"" short:"d" value:"none"`
	AssumeInstalled []string `pflag:"" long:"assume-installed" value:"required"`
	DbOnly          bool     `pflag:"" value:"none"`
	NoProgressBar   bool     `pflag:"" value:"none"`
	NoScriptlet     bool     `pflag:"" value:"none"`
	Print           bool     `pflag:"" short:"p" value:"none"`
	PrintFormat     string   `pflag:"" long:"print-format" short:"p" value:"rquired"`
	AsDeps          bool     `pflag:"" value:"none"`
	AsExplicit      bool     `pflag:"" value:"none"`
	Needed          bool     `pflag:"" value:"None"`
	Overwrite       []string `pflag:"" value:"required" split:","`
	ChangeLog       bool     `pflag:"" short:"c" value:"none"`
	Deps            bool     `pflag:"" short:"d" value:"none"`
	Explicit        bool     `pflag:"" short:"e" value:"none"`
	Groups          bool     `pflag:"" short:"g" value:"none"`
	Info            int      `pflag:"" short:"i" value:"none"`
	Check           int      `pflag:"" short:"k" value:"none"`
	List            bool     `pflag:"" short:"l" value:"none"`
	Foreign         bool     `pflag:"" short:"m" value:"none"`
	Native          bool     `pflag:"" short:"n" value:"none"`
	Owns            bool     `pflag:"" short:"o" value:"none"`
	File            bool     `pflag:"" short:"p" value:"none"`
	Quiet           bool     `pflag:"" short:"q" value:"none"`
	Search          bool     `pflag:"" short:"s" value:"none"`
	Unrequired      bool     `pflag:"" short:"t" value:"none"`
	Upgrades        bool     `pflag:"" short:"u" value:"none"`
	Cascase         bool     `pflag:"" short:"c" value:"none"`
	NoSave          bool     `pflag:"" short:"n" value:"none"`
	Recursive       bool     `pflag:"" short:"s" value:"none"`
	Unneeded        bool     `pflag:"" short:"u" value:"none"`
	Clean           int      `pflag:"" short:"c" value:"none"`
	SysUpgrade      int      `pflag:"" short:"u" value:"none"`
	DownloadOnly    bool     `pflag:"" short:"w" value:"none"`
	Refresh         int      `pflag:"" short:"y" value:"none"`
	Regex           bool     `pflag:"" short:"x" value:"none"`
	MachineReadable bool     `pflag:"" value:"none"`
	Help            bool     `pflag:"" short:"h" value:"none"`

	SearchMode     int
	HideMenus      bool
	VCSPath        string
	CompletionPath string
	ConfigPath     string

	Alpm   *alpm.Handle
	Pacman *pacmanconf.Config

	NoConfirm bool

	PacmanFlags []Arg
	Targets     []string
}

const SystemConfigFile = "/etc/yay.conf"

func (config *YayConfig) NeedRoot() bool {
	if config.Help {
		return false
	}

	switch config.Op {
	case "D", "database":
		if config.Check > 0 {
			return false
		}
		return true
	case "F", "files":
		if config.Refresh > 0 {
			return true
		}
		return false
	case "Q", "query":
		if config.Check > 0 {
			return true
		}
		return false
	case "R", "remove":
		if config.Print || config.PrintFormat != "" {
			return false
		}
		return true
	case "S", "sync":
		if config.Refresh > 0 {
			return true
		}
		if config.Print || config.PrintFormat != "" {
			return false
		}
		if config.Search {
			return false
		}
		if config.List {
			return false
		}
		if config.Groups {
			return false
		}
		if config.Info > 0 {
			return false
		}
		if config.Clean > 0 && config.Mode == ModeAUR {
			return false
		}
		return true
	case "U", "upgrade":
		return true
	default:
		return false
	}
}

func (conf *YayConfig) LoadFile(file string) (bool, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, ini.ParseFile(file, parseCallback, conf)
}

func (conf *YayConfig) AddTarget(arg string) {
	conf.Targets = append(conf.Targets, arg)
}

func (conf *YayConfig) ParseStdin() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		conf.AddTarget(scanner.Text())
	}

	return os.Stdin.Close()
}

func DefaultConfig() *YayConfig {
	config := &YayConfig{
		AURURL: "https://aur.archlinux.org",

		Redownload: "no",
		Rebuild:    "no",
		RemoveMake: "no",
		SortBy:     "votes",
		SortMode:   "bottomup",
		Mode:       "any",

		RequestSplitN:      150,
		CompletionInterval: 7,

		Editor:     "vim",
		MakepkgBin: "makepkg",
		PacmanBin:  "pacman",
		TarBin:     "bsdtar",
		GitBin:     "git",
		GpgBin:     "gpg",
		SudoBin:    "sudo",

		MakepkgConf: "",
		PacmanConf:  "/etc/pacman.conf",

		SearchBy: "name-desc",
	}

	config.setPaths()

	return config
}

func (conf *YayConfig) globalArgs() (args []Arg) {
	val := reflect.ValueOf(conf).Elem()

	for i := 0; i < val.NumField(); i++ {
		tag := val.Type().Field(i).Tag
		if _, ok := tag.Lookup("global"); ok {
			args = append(args, formatVar(val, i)...)
		}
	}

	return
}

func formatVar(val reflect.Value, i int) []Arg {
	field := val.Field(i)
	tag := val.Type().Field(i).Tag
	name := strings.ToLower(val.Type().Field(i).Name)

	if _, ok := tag.Lookup("long"); ok {
		name = tag.Get("long")
	}

	switch field.Kind() {
	case reflect.Bool:
		if field.Bool() {
			return []Arg{{Arg: name}}
		}
	case reflect.String:
		if field.String() != "" {
			return []Arg{{name, field.String()}}
		}
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			arr := []Arg{}
			for i := 0; i < field.Len(); i++ {
				arr = append(arr, Arg{name, field.Index(i).String()})
			}
			return arr
		}

	}

	return nil
}

func (conf *YayConfig) hasParam(arg string) bool {
	val := reflect.ValueOf(conf).Elem()

	for i := 0; i < val.NumField(); i++ {
		name := strings.ToLower(val.Type().Field(i).Name)
		tag := val.Type().Field(i).Tag

		if arg == name || arg == tag.Get("long") {
			if tag.Get("value") == "required" {
				return true
			}
		}
	}

	return false

}

func initDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return errors.New(gotext.Get("failed to create config directory '%s': %s", dir, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}

func (config *YayConfig) setPaths() error {
	cacheHome := ""
	configHome := ""

	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config", "yay")
	} else {
		return errors.New(gotext.Get("%s and %s unset", "XDG_CONFIG_HOME", "HOME"))
	}

	if err := initDir(configHome); err != nil {
		return err
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, "yay")
	} else if cacheHome = os.Getenv("HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, ".cache", "yay")
	} else {
		return errors.New(gotext.Get("%s and %s unset", "XDG_CACHE_HOME", "HOME"))
	}

	if err := initDir(cacheHome); err != nil {
		return err
	}

	config.ConfigPath = filepath.Join(configHome, configFileName)
	config.VCSPath = filepath.Join(cacheHome, vcsFileName)
	config.CompletionPath = filepath.Join(cacheHome, completionFileName)
	config.BuildDir = filepath.Join(cacheHome, "pkg")
	config.ABSDir = filepath.Join(cacheHome, "abs")

	aurdest := os.Getenv("AURDEST")
	if aurdest != "" {
		config.BuildDir = aurdest
	}

	return nil
}

func allowedValues(tag reflect.StructTag) []string {
	if tagValue, ok := tag.Lookup("values"); ok {
		return strings.Split(tagValue, ",")
	}

	return nil
}

func valueAllows(value string, tag reflect.StructTag) bool {
	allowed := allowedValues(tag)
	for _, v := range allowed {
		if v == value {
			return true
		}
	}
	return len(allowed) == 0
}

func expected(key string, value string, tag reflect.StructTag) InvalidOption {
	tagValue, ok := tag.Lookup("values")
	if !ok {
		return InvalidOption{key, value, nil}
	}
	return InvalidOption{key, value, strings.Split(tagValue, ",")}
}

func parseCallback(fileName string, line int, section string,
	key string, value string, data interface{}) error {
	if line < 0 {
		return fmt.Errorf("unable to read file: %s: %s", fileName, section)
	}
	if key == "" && value == "" {
		return nil
	}
	if key == "Include" {
		return ini.ParseFile(value, parseCallback, data)
	}

	value = os.ExpandEnv(value)
	conf := data.(*YayConfig)

	if err := conf.setOption(key, value, section); err != nil {
		return fmt.Errorf("%s:%d: %s", fileName, line, err.Error())
	}

	return nil
}

func (conf *YayConfig) setOption(key string, value string, section string) error {
	return conf._setOption(key, value, section, false)
}

func (conf *YayConfig) setOp(op string) error {
	if conf.Op != "" {
		return errors.New(gotext.Get("only one operation can be used at a time"))
	}

	conf.Op = op

	return nil
}

func (conf *YayConfig) setFlag(key string, value string) error {
	switch key {
	case "noconfirm":
		conf.NoConfirm = true
	case "confirm":
		conf.NoConfirm = false
	case "v", "version":
		return conf.setOp(Version)
	case "D", "database":
		return conf.setOp(Database)
	case "F", "files":
		return conf.setOp(Files)
	case "Q", "query":
		return conf.setOp(Query)
	case "R", "remove":
		return conf.setOp(Remove)
	case "S", "sync":
		return conf.setOp(Sync)
	case "T", "deptest":
		return conf.setOp(DepTest)
	case "U", "upgrade":
		return conf.setOp(Upgrade)
	// yay specific
	case "Y", "yay":
		return conf.setOp(Yay)
	case "P", "show":
		return conf.setOp(Show)
	case "G", "getpkgbuild":
		return conf.setOp(GetPkgbuild)
	default:
		return conf._setOption(key, value, "", true)
	}

	return nil
}

func (conf *YayConfig) _setOption(key string, value string, section string, flag bool) error {
	val := reflect.ValueOf(conf).Elem()
	found := false
	for i := 0; i < val.NumField(); i++ {
		err, ok := conf.handleVal(key, value, section, flag, val, i)
		found = found || ok
		if err != nil {
			return err
		}
	}

	if !found {
		return UnknownOption{key}
	}

	return nil
}

func (conf *YayConfig) handleVal(key string, value string, section string, flag bool, val reflect.Value, i int) (error, bool) {
	field := val.Field(i)
	tag := val.Type().Field(i).Tag
	name := strings.ToLower(val.Type().Field(i).Name)
	no := false

	key = strings.ToLower(key)

	if flag {
		no = strings.HasPrefix(key, "no")
		if _, ok := tag.Lookup("no"); no && !ok {
			return UnknownOption{key}, true
		}
		key = strings.TrimPrefix(key, "no")
	}

	if _, ok := tag.Lookup("section"); !ok {
		_, ok := tag.Lookup("pflag")
		_, ok2 := tag.Lookup("flag")

		if !ok && !ok2 {
			return nil, false
		}
	}

	_, alias := tag.Lookup("alias")
	short, okShort := tag.Lookup("short")

	tagName, ok := tag.Lookup("long")
	if (!alias && (!ok && key == name) || (ok && key == tagName)) || (alias && valueAllows(key, tag)) || (okShort && key == short) {
		if alias {
			if value != "" {
				return InvalidOption{key: key, value: value}, true
			}
			value = key
		}

		if no {
			if value != "" {
				return InvalidOption{"no" + key, value, nil}, true
			}
		} else if (tag.Get("value") == "required" && value == "") ||
			(tag.Get("value") == "none" && value != "") {
			return expected(key, value, tag), true
		}

		if allowed := allowedValues(tag); value == "" && len(allowed) > 0 {
			value = allowed[0]
		}

		if _, ok := tag.Lookup("case"); !ok {
			if !valueAllows(strings.ToLower(value), tag) {
				return expected(key, value, tag), true
			}
		} else {
			if !valueAllows(value, tag) {
				return expected(key, value, tag), true
			}
		}

		//flags can't be in a section so ignore the section
		if !flag && tag.Get("section") != section {
			return fmt.Errorf("option '%s' does not belong in section %s", key, section), true
			//return nil, false
		}

		return conf.setVal(key, value, field, tag, flag, no), true
	}

	return nil, false
}

func (conf *YayConfig) Globals() *Args {
	return &Args{conf.Op, conf.globalArgs(), nil}
}

func (conf *YayConfig) Flags() *Args {
	args := append([]Arg{}, conf.globalArgs()...)
	args = append(args, conf.PacmanFlags...)
	return &Args{"-" + conf.Op, args, conf.Targets}
}

func (conf *YayConfig) setVal(key string, value string, field reflect.Value, tag reflect.StructTag, flag bool, no bool) error {
	if _, ok := tag.Lookup("pflag"); ok {
		if _, ok := tag.Lookup("global"); !ok {
			conf.PacmanFlags = append(conf.PacmanFlags, Arg{key, value})
		}
	}

	switch field.Kind() {
	case reflect.Int:
		if no {
			return UnknownOption{key}
		}
		if value == "" {
			field.SetInt(field.Int() + 1)
		} else {
			num, err := strconv.Atoi(value)
			if err != nil {
				return InvalidOption{key, value, []string{"number"}}
			}
			field.SetInt(int64(num))
		}
	case reflect.Bool:
		field.SetBool(!no)
	case reflect.String:
		if no {
			allowed := allowedValues(tag)
			if len(allowed) >= 2 {
				field.SetString(allowed[1])
			} else {
				field.SetString("")
			}
		} else {
			field.SetString(value)
		}
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			if no {
				field.Set(reflect.ValueOf([]string{}))
			} else if field.IsNil() {
				if split, ok := tag.Lookup("split"); ok {
					if split == "" {
						field.Set(reflect.ValueOf(strings.Fields(value)))
					} else {
						field.Set(reflect.ValueOf(strings.Split(value, split)))
					}
				} else {
					field.Set(reflect.ValueOf([]string{value}))
				}
			} else {
				if split, ok := tag.Lookup("split"); ok {
					if split == "" {
						field.Set(reflect.AppendSlice(field, reflect.ValueOf(strings.Fields(value))))
					} else {
						field.Set(reflect.AppendSlice(field, reflect.ValueOf(strings.Split(value, split))))
					}
				} else {
					field.Set(reflect.ValueOf([]string{value}))
				}
			}
		}
	}

	return nil
}
