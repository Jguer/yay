package main // import "github.com/Jguer/yay"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	alpm "github.com/Jguer/go-alpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	gotext.Configure(localePath, os.Getenv("LANG"), "yay")
}

func initConfig(configPath string) error {
	cfile, err := os.Open(configPath)
	if !os.IsNotExist(err) && err != nil {
		return errors.New(gotext.Get("failed to open config file '%s': %s", configPath, err))
	}

	defer cfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(cfile)
		if err = decoder.Decode(&config); err != nil {
			return errors.New(gotext.Get("failed to read config file '%s': %s", configPath, err))
		}
	}

	aurdest := os.Getenv("AURDEST")
	if aurdest != "" {
		config.BuildDir = aurdest
	}

	return nil
}

func initVCS(vcsFilePath string) error {
	vfile, err := os.Open(vcsFilePath)
	if !os.IsNotExist(err) && err != nil {
		return errors.New(gotext.Get("failed to open vcs file '%s': %s", vcsFilePath, err))
	}

	defer vfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&savedInfo); err != nil {
			return errors.New(gotext.Get("failed to read vcs file '%s': %s", vcsFilePath, err))
		}
	}

	return nil
}

func initBuildDir() error {
	if _, err := os.Stat(config.BuildDir); os.IsNotExist(err) {
		if err = os.MkdirAll(config.BuildDir, 0o755); err != nil {
			return errors.New(gotext.Get("failed to create BuildDir directory '%s': %s", config.BuildDir, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initAlpm(cmdArgs *settings.Arguments, pacmanConfigPath string) (*alpm.Handle, *pacmanconf.Config, error) {
	root := "/"
	if value, _, exists := cmdArgs.GetArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err := pacmanconf.PacmanConf("--config", pacmanConfigPath, "--root", root)
	if err != nil {
		return nil, nil, fmt.Errorf("%s", stderr)
	}

	if dbPath, _, exists := cmdArgs.GetArg("dbpath", "b"); exists {
		pacmanConf.DBPath = dbPath
	}

	if arch, _, exists := cmdArgs.GetArg("arch"); exists {
		pacmanConf.Architecture = arch
	}

	if ignoreArray := cmdArgs.GetArgs("ignore"); ignoreArray != nil {
		pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, ignoreArray...)
	}

	if ignoreGroupsArray := cmdArgs.GetArgs("ignoregroup"); ignoreGroupsArray != nil {
		pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, ignoreGroupsArray...)
	}

	if cacheArray := cmdArgs.GetArgs("cachedir"); cacheArray != nil {
		pacmanConf.CacheDir = cacheArray
	}

	if gpgDir, _, exists := cmdArgs.GetArg("gpgdir"); exists {
		pacmanConf.GPGDir = gpgDir
	}

	alpmHandle, err := initAlpmHandle(pacmanConf, nil)
	if err != nil {
		return nil, nil, err
	}

	switch value, _, _ := cmdArgs.GetArg("color"); value {
	case "always":
		text.UseColor = true
	case "auto":
		text.UseColor = isTty()
	case "never":
		text.UseColor = false
	default:
		text.UseColor = pacmanConf.Color && isTty()
	}

	return alpmHandle, pacmanConf, nil
}

func initAlpmHandle(pacmanConf *pacmanconf.Config, oldAlpmHandle *alpm.Handle) (*alpm.Handle, error) {
	if oldAlpmHandle != nil {
		if errRelease := oldAlpmHandle.Release(); errRelease != nil {
			return nil, errRelease
		}
	}

	alpmHandle, err := alpm.Initialize(pacmanConf.RootDir, pacmanConf.DBPath)
	if err != nil {
		return nil, errors.New(gotext.Get("unable to CreateHandle: %s", err))
	}

	if err := configureAlpm(pacmanConf, alpmHandle); err != nil {
		return nil, err
	}

	alpmHandle.SetQuestionCallback(questionCallback)
	alpmHandle.SetLogCallback(logCallback)
	return alpmHandle, nil
}

func exitOnError(err error) {
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		cleanup(config.Runtime.AlpmHandle)
		os.Exit(1)
	}
}

func cleanup(alpmHandle *alpm.Handle) int {
	if alpmHandle != nil {
		if err := alpmHandle.Release(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	return 0
}

func main() {
	initGotext()
	if os.Geteuid() == 0 {
		text.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	cmdArgs := settings.MakeArguments()
	runtime, err := settings.MakeRuntime()
	exitOnError(err)
	config = settings.MakeConfig()
	config.Runtime = runtime
	exitOnError(initConfig(runtime.ConfigPath))
	exitOnError(cmdArgs.ParseCommandLine(config))
	if config.Runtime.SaveConfig {
		errS := config.SaveConfig(runtime.ConfigPath)
		if errS != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	config.ExpandEnv()
	exitOnError(initBuildDir())
	exitOnError(initVCS(runtime.VCSPath))
	config.Runtime.AlpmHandle, config.Runtime.PacmanConf, err = initAlpm(cmdArgs, config.PacmanConf)
	exitOnError(err)
	exitOnError(handleCmd(cmdArgs, config.Runtime.AlpmHandle))
	os.Exit(cleanup(config.Runtime.AlpmHandle))
}
