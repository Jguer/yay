package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Jguer/yay/v10/pkg/types"
	rpc "github.com/mikkeloscar/aur"
)

// Verbosity settings for search
const (
	NumberMenu = iota
	Detailed
	Minimal
)

const (
	// Describes Sorting method for numberdisplay
	BottomUp = iota
	TopDown
)

var Version = "9.2.1"

// Configuration handles program config
type Configuration struct {
	AURURL             string `json:"aururl"`
	BuildDir           string `json:"buildDir"`
	Editor             string `json:"editor"`
	EditorFlags        string `json:"editorflags"`
	MakepkgBin         string `json:"makepkgbin"`
	MakepkgConf        string `json:"makepkgconf"`
	PacmanBin          string `json:"pacmanbin"`
	PacmanConf         string `json:"pacmanconf"` // Rename to PacmanConfPath soon
	TarBin             string `json:"tarbin"`
	ReDownload         string `json:"redownload"`
	ReBuild            string `json:"rebuild"`
	BatchInstall       bool   `json:"batchinstall"`
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

	Mode             types.TargetMode `json:"-"`
	HideMenus        bool             `json:"-"`
	ShouldSaveConfig bool             `json:"-"`
}

// ExpandEnv expands environment variables in the configuration structure
func (config *Configuration) ExpandEnv() {
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

func (config *Configuration) String() string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "\t")
	if err := enc.Encode(config); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return buf.String()
}

func DefaultSettings() *Configuration {
	config := &Configuration{
		AURURL:             "https://aur.archlinux.org",
		BuildDir:           "$HOME/.cache/yay",
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
		SudoLoop:           false,
		TarBin:             "bsdtar",
		GitBin:             "git",
		GpgBin:             "gpg",
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
		GitClone:           true,
		Provides:           true,
		UpgradeMenu:        true,
		CleanMenu:          true,
		DiffMenu:           true,
		EditMenu:           false,
		UseAsk:             false,
		CombinedUpgrade:    false,
		// Missing new components
		Mode: types.Any,
	}

	if os.Getenv("XDG_CACHE_HOME") != "" {
		config.BuildDir = "$XDG_CACHE_HOME/yay"
	}

	return config
}

// SaveConfig writes yay config to file.
func (config *Configuration) SaveConfig(configFilePath string) error {
	marshalledinfo, _ := json.MarshalIndent(config, "", "\t")
	in, err := os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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

func (config *Configuration) handleConfig(option, value string) bool {
	switch option {
	case "aururl":
		config.AURURL = value
	case "save":
		config.ShouldSaveConfig = true
	case "afterclean", "cleanafter":
		config.CleanAfter = true
	case "noafterclean", "nocleanafter":
		config.CleanAfter = false
	case "devel":
		config.Devel = true
	case "nodevel":
		config.Devel = false
	case "timeupdate":
		config.TimeUpdate = true
	case "notimeupdate":
		config.TimeUpdate = false
	case "topdown":
		config.SortMode = TopDown
	case "bottomup":
		config.SortMode = BottomUp
	case "completioninterval":
		n, err := strconv.Atoi(value)
		if err == nil {
			config.CompletionInterval = n
		}
	case "sortby":
		config.SortBy = value
	case "noconfirm":
		config.NoConfirm = true
	case "config":
		config.PacmanConf = value
	case "redownload":
		config.ReDownload = "yes"
	case "redownloadall":
		config.ReDownload = "all"
	case "noredownload":
		config.ReDownload = "no"
	case "rebuild":
		config.ReBuild = "yes"
	case "rebuildall":
		config.ReBuild = "all"
	case "rebuildtree":
		config.ReBuild = "tree"
	case "norebuild":
		config.ReBuild = "no"
	case "batchinstall":
		config.BatchInstall = true
	case "nobatchinstall":
		config.BatchInstall = false
	case "answerclean":
		config.AnswerClean = value
	case "noanswerclean":
		config.AnswerClean = ""
	case "answerdiff":
		config.AnswerDiff = value
	case "noanswerdiff":
		config.AnswerDiff = ""
	case "answeredit":
		config.AnswerEdit = value
	case "noansweredit":
		config.AnswerEdit = ""
	case "answerupgrade":
		config.AnswerUpgrade = value
	case "noanswerupgrade":
		config.AnswerUpgrade = ""
	case "gitclone":
		config.GitClone = true
	case "nogitclone":
		config.GitClone = false
	case "gpgflags":
		config.GpgFlags = value
	case "mflags":
		config.MFlags = value
	case "gitflags":
		config.GitFlags = value
	case "builddir":
		config.BuildDir = value
	case "editor":
		config.Editor = value
	case "editorflags":
		config.EditorFlags = value
	case "makepkg":
		config.MakepkgBin = value
	case "makepkgconf":
		config.MakepkgConf = value
	case "nomakepkgconf":
		config.MakepkgConf = ""
	case "pacman":
		config.PacmanBin = value
	case "tar":
		config.TarBin = value
	case "git":
		config.GitBin = value
	case "gpg":
		config.GpgBin = value
	case "requestsplitn":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			config.RequestSplitN = n
		}
	case "sudoloop":
		config.SudoLoop = true
	case "nosudoloop":
		config.SudoLoop = false
	case "provides":
		config.Provides = true
	case "noprovides":
		config.Provides = false
	case "pgpfetch":
		config.PGPFetch = true
	case "nopgpfetch":
		config.PGPFetch = false
	case "upgrademenu":
		config.UpgradeMenu = true
	case "noupgrademenu":
		config.UpgradeMenu = false
	case "cleanmenu":
		config.CleanMenu = true
	case "nocleanmenu":
		config.CleanMenu = false
	case "diffmenu":
		config.DiffMenu = true
	case "nodiffmenu":
		config.DiffMenu = false
	case "editmenu":
		config.EditMenu = true
	case "noeditmenu":
		config.EditMenu = false
	case "useask":
		config.UseAsk = true
	case "nouseask":
		config.UseAsk = false
	case "combinedupgrade":
		config.CombinedUpgrade = true
	case "nocombinedupgrade":
		config.CombinedUpgrade = false
	case "a", "aur":
		config.Mode = types.AUR
	case "repo":
		config.Mode = types.Repo
	case "removemake":
		config.RemoveMake = "yes"
	case "noremovemake":
		config.RemoveMake = "no"
	case "askremovemake":
		config.RemoveMake = "ask"
	default:
		return false
	}

	return true
}

func (config *Configuration) ParseCommandLine() (*types.Arguments, error) {
	args := os.Args[1:]
	usedNext := false
	parser := types.MakeArguments()
	var err error

	if len(args) < 1 {
		parser.ParseShortOption("-Syu", "")
	} else {
		for k, arg := range args {
			var nextArg string

			if usedNext {
				usedNext = false
				continue
			}

			if k+1 < len(args) {
				nextArg = args[k+1]
			}

			switch {
			case parser.ExistsArg("--"):
				parser.AddTarget(arg)
			case strings.HasPrefix(arg, "--"):
				usedNext, err = parser.ParseLongOption(arg, nextArg)
			case strings.HasPrefix(arg, "-"):
				usedNext, err = parser.ParseShortOption(arg, nextArg)
			default:
				parser.AddTarget(arg)
			}

			if err != nil {
				return nil, err
			}
		}
	}

	if parser.Op == "" {
		parser.Op = "Y"
	}

	if parser.ExistsArg("-") {
		var file *os.File
		err = parser.ParseStdin()
		parser.DelArg("-")

		if err != nil {
			return nil, err
		}

		file, err = os.Open("/dev/tty")

		if err != nil {
			return nil, err
		}

		os.Stdin = file
	}

	config.extractYayOptions(parser)
	return parser, nil
}

func (config *Configuration) extractYayOptions(parser *types.Arguments) {
	for option, value := range parser.Options {
		if config.handleConfig(option, value) {
			parser.DelArg(option)
		}
	}

	for option, value := range parser.Globals {
		if config.handleConfig(option, value) {
			parser.DelArg(option)
		}
	}

	rpc.AURURL = strings.TrimRight(config.AURURL, "/") + "/rpc.php?"
	config.AURURL = strings.TrimRight(config.AURURL, "/")
}
