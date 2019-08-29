package runtime

import (
	"fmt"
	"strings"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Morganamilo/go-pacmanconf"
)

func InitAlpmHandle(config *Configuration, pacmanConf *pacmanconf.Config, oldHandle *alpm.Handle) (*alpm.Handle, error) {
	var err error

	var alpmHandle *alpm.Handle

	if oldHandle == nil {
		// There's no old handle so return a new one
		alpmHandle = new(alpm.Handle)
		if alpmHandle, err = alpm.Initialize(pacmanConf.RootDir, pacmanConf.DBPath); err != nil {
			return nil, fmt.Errorf("Unable to CreateHandle: %s", err)
		}
	} else {
		// There's an old handle so just reopen the pointer inside
		if err := oldHandle.Reopen(); err != nil {
			return nil, err
		}
		alpmHandle = oldHandle
	}

	if err := configureAlpm(pacmanConf, alpmHandle); err != nil {
		return nil, err
	}

	alpmHandle.SetQuestionCallback(func(question alpm.QuestionAny) {
		callbackQuestion(config, alpmHandle, question)
	})

	alpmHandle.SetLogCallback(callbackLog)
	return alpmHandle, nil
}

func InitPacmanConf(cmdArgs *types.Arguments, pacmanConfFile string) (*pacmanconf.Config, error) {
	var err error
	var stderr string

	root := "/"
	if value, _, exists := cmdArgs.GetArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err := pacmanconf.PacmanConf("--config", pacmanConfFile, "--root", root)
	if err != nil {
		return nil, fmt.Errorf("%s", stderr)
	}

	if value, _, exists := cmdArgs.GetArg("dbpath", "b"); exists {
		pacmanConf.DBPath = value
	}

	if value, _, exists := cmdArgs.GetArg("arch"); exists {
		pacmanConf.Architecture = value
	}

	if value, _, exists := cmdArgs.GetArg("ignore"); exists {
		pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, strings.Split(value, ",")...)
	}

	if value, _, exists := cmdArgs.GetArg("ignoregroup"); exists {
		pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	//TODO
	//current system does not allow duplicate arguments
	//but pacman allows multiple cachedirs to be passed
	//for now only handle one cache dir
	if value, _, exists := cmdArgs.GetArg("cachedir"); exists {
		pacmanConf.CacheDir = []string{value}
	}

	if value, _, exists := cmdArgs.GetArg("gpgdir"); exists {
		pacmanConf.GPGDir = value
	}

	return pacmanConf, nil
}

func configureAlpm(pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle) error {

	// TODO: set SigLevel
	//sigLevel := alpm.SigPackage | alpm.SigPackageOptional | alpm.SigDatabase | alpm.SigDatabaseOptional
	//localFileSigLevel := alpm.SigUseDefault
	//remoteFileSigLevel := alpm.SigUseDefault

	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		db, err := alpmHandle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return err
		}

		db.SetServers(repo.Servers)
		db.SetUsage(toUsage(repo.Usage))

	}

	if err := alpmHandle.SetCacheDirs(pacmanConf.CacheDir); err != nil {
		return err
	}

	// add hook directories 1-by-1 to avoid overwriting the system directory
	for _, dir := range pacmanConf.HookDir {
		if err := alpmHandle.AddHookDir(dir); err != nil {
			return err
		}
	}

	if err := alpmHandle.SetGPGDir(pacmanConf.GPGDir); err != nil {
		return err
	}

	if err := alpmHandle.SetLogFile(pacmanConf.LogFile); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnorePkgs(pacmanConf.IgnorePkg); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnoreGroups(pacmanConf.IgnoreGroup); err != nil {
		return err
	}

	if err := alpmHandle.SetArch(pacmanConf.Architecture); err != nil {
		return err
	}

	if err := alpmHandle.SetNoUpgrades(pacmanConf.NoUpgrade); err != nil {
		return err
	}

	if err := alpmHandle.SetNoExtracts(pacmanConf.NoExtract); err != nil {
		return err
	}

	/*if err := alpmHandle.SetDefaultSigLevel(sigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetLocalFileSigLevel(localFileSigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetRemoteFileSigLevel(remoteFileSigLevel); err != nil {
		return err
	}*/

	if err := alpmHandle.SetDeltaRatio(pacmanConf.UseDelta); err != nil {
		return err
	}

	if err := alpmHandle.SetUseSyslog(pacmanConf.UseSyslog); err != nil {
		return err
	}

	return alpmHandle.SetCheckSpace(pacmanConf.CheckSpace)
}

func toUsage(usages []string) alpm.Usage {
	if len(usages) == 0 {
		return alpm.UsageAll
	}

	var ret alpm.Usage
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
