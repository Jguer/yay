package settings

import (
	"strconv"
	"strings"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/leonelquinteros/gotext"
)

func (c *Configuration) ParseCommandLine(a *parser.Arguments, logger *text.Logger) error {
	if err := a.Parse(); err != nil {
		return err
	}

	c.extractYayOptions(a, logger)

	return nil
}

func (c *Configuration) extractYayOptions(a *parser.Arguments, logger *text.Logger) {
	for option, value := range a.Options {
		if c.handleOption(option, value.First(), logger) {
			a.DelArg(option)
		}
	}

	c.AURURL = strings.TrimRight(c.AURURL, "/")

	// if AurRPCURL is set, use that for /rpc calls
	if c.AURRPCURL == "" {
		c.AURRPCURL = c.AURURL + "/rpc?"
		return
	}

	if !strings.HasSuffix(c.AURRPCURL, "?") {
		if strings.HasSuffix(c.AURRPCURL, "/rpc") {
			c.AURRPCURL += "?"
		} else {
			c.AURRPCURL = strings.TrimRight(c.AURRPCURL, "/") + "/rpc?"
		}
	}
}

func (c *Configuration) handleOption(option, value string, logger *text.Logger) bool {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		boolValue = true
	}

	switch option {
	case "aururl":
		c.AURURL = value
	case "aurrpcurl":
		c.AURRPCURL = value
	case "save":
		c.SaveConfig = boolValue
	case "afterclean", "cleanafter":
		c.CleanAfter = boolValue
	case "noafterclean", "nocleanafter":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--cleanafter=false' instead", option))
		c.CleanAfter = !boolValue
	case "keepsrc":
		c.KeepSrc = boolValue
	case "debug":
		c.Debug = boolValue
		return !boolValue
	case "devel":
		c.Devel = boolValue
	case "nodevel":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--devel=false' instead", option))
		c.Devel = !boolValue
	case "timeupdate":
		c.TimeUpdate = boolValue
	case "notimeupdate":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--timeupdate=false' instead", option))
		c.TimeUpdate = !boolValue
	case "topdown":
		c.BottomUp = false
	case "bottomup":
		c.BottomUp = true
	case "singlelineresults":
		c.SingleLineResults = true
	case "doublelineresults":
		c.SingleLineResults = false
	case "completioninterval":
		n, err := strconv.Atoi(value)
		if err == nil {
			c.CompletionInterval = n
		}
	case "sortby":
		c.SortBy = value
	case "searchby":
		c.SearchBy = value
	case "noconfirm":
		NoConfirm = boolValue
	case "config":
		c.PacmanConf = value
	case "redownload":
		c.ReDownload = "yes"
	case "redownloadall":
		c.ReDownload = "all"
	case "noredownload":
		c.ReDownload = "no"
	case "rebuild":
		c.ReBuild = parser.RebuildModeYes
	case "rebuildall":
		c.ReBuild = parser.RebuildModeAll
	case "rebuildtree":
		c.ReBuild = parser.RebuildModeTree
	case "norebuild":
		c.ReBuild = parser.RebuildModeNo
	case "batchinstall":
		c.BatchInstall = boolValue
	case "nobatchinstall":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--batchinstall=false' instead", option))
		c.BatchInstall = !boolValue
	case "answerclean":
		c.AnswerClean = value
	case "noanswerclean":
		c.AnswerClean = ""
	case "answerdiff":
		c.AnswerDiff = value
	case "noanswerdiff":
		c.AnswerDiff = ""
	case "answeredit":
		c.AnswerEdit = value
	case "noansweredit":
		c.AnswerEdit = ""
	case "answerupgrade":
		c.AnswerUpgrade = value
	case "noanswerupgrade":
		c.AnswerUpgrade = ""
	case "gpgflags":
		c.GpgFlags = value
	case "mflags":
		c.MFlags = value
	case "gitflags":
		c.GitFlags = value
	case "builddir":
		c.BuildDir = value
	case "editor":
		c.Editor = value
	case "editorflags":
		c.EditorFlags = value
	case "makepkg":
		c.MakepkgBin = value
	case "makepkgconf":
		c.MakepkgConf = value
	case "nomakepkgconf":
		c.MakepkgConf = ""
	case "pacman":
		c.PacmanBin = value
	case "git":
		c.GitBin = value
	case "gpg":
		c.GpgBin = value
	case "sudo":
		c.SudoBin = value
	case "sudoflags":
		c.SudoFlags = value
	case "requestsplitn":
		n, err := strconv.Atoi(value)
		if err == nil && n > 0 {
			c.RequestSplitN = n
		}
	case "sudoloop":
		c.SudoLoop = boolValue
	case "nosudoloop":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--sudoloop=false' instead", option))
		c.SudoLoop = !boolValue
	case "provides":
		c.Provides = boolValue
	case "noprovides":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--provides=false' instead", option))
		c.Provides = !boolValue
	case "pgpfetch":
		c.PGPFetch = boolValue
	case "nopgpfetch":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--pgpfetch=false' instead", option))
		c.PGPFetch = !boolValue
	case "cleanmenu":
		c.CleanMenu = boolValue
	case "nocleanmenu":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--cleanmenu=false' instead", option))
		c.CleanMenu = !boolValue
	case "diffmenu":
		c.DiffMenu = boolValue
	case "nodiffmenu":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--diffmenu=false' instead", option))
		c.DiffMenu = !boolValue
	case "editmenu":
		c.EditMenu = boolValue
	case "noeditmenu":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--editmenu=false' instead", option))
		c.EditMenu = !boolValue
	case "useask":
		c.UseAsk = boolValue
	case "nouseask":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--useask=false' instead", option))
		c.UseAsk = !boolValue
	case "combinedupgrade":
		c.CombinedUpgrade = boolValue
	case "nocombinedupgrade":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--combinedupgrade=false' instead", option))
		c.CombinedUpgrade = !boolValue
	case "a", "aur":
		c.Mode = parser.ModeAUR
	case "z", "repo":
		c.Mode = parser.ModeRepo
	case "removemake":
		c.RemoveMake = "yes"
	case "noremovemake":
		c.RemoveMake = "no"
	case "askremovemake":
		c.RemoveMake = "ask"
	case "askyesremovemake":
		c.RemoveMake = "askyes"
	case "separatesources":
		c.SeparateSources = boolValue
	case "noseparatesources":
		logger.Warnln(gotext.Get("'--%s' is deprecated. Use '--separatesources=false' instead", option))
		c.SeparateSources = !boolValue
	default:
		return false
	}

	return true
}
