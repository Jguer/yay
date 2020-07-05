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
	in, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
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
