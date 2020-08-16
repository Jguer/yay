package main // import "github.com/Jguer/yay"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/db/ialpm"
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

func initAlpm(cmdArgs *settings.Arguments, pacmanConfigPath string) (*pacmanconf.Config, bool, error) {
	root := "/"
	if value, _, exists := cmdArgs.GetArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err := pacmanconf.PacmanConf("--config", pacmanConfigPath, "--root", root)
	if err != nil {
		return nil, false, fmt.Errorf("%s", stderr)
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

	useColor := pacmanConf.Color && isTty()
	switch value, _, _ := cmdArgs.GetArg("color"); value {
	case "always":
		useColor = true
	case "auto":
		useColor = isTty()
	case "never":
		useColor = false
	}

	return pacmanConf, useColor, nil
}

func main() {
	ret := 0
	defer func() { os.Exit(ret) }()
	initGotext()
	if os.Geteuid() == 0 {
		text.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	cmdArgs := settings.MakeArguments()
	runtime, err := settings.MakeRuntime()
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	config = settings.MakeConfig()
	config.Runtime = runtime

	err = initConfig(runtime.ConfigPath)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	err = cmdArgs.ParseCommandLine(config)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	if config.Runtime.SaveConfig {
		errS := config.SaveConfig(runtime.ConfigPath)
		if errS != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	config.ExpandEnv()
	err = initBuildDir()
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	err = initVCS(runtime.VCSPath)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}
	var useColor bool
	config.Runtime.PacmanConf, useColor, err = initAlpm(cmdArgs, config.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	text.UseColor = useColor

	dbExecutor, err := ialpm.NewExecutor(runtime.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	defer dbExecutor.Cleanup()
	err = handleCmd(cmdArgs, db.Executor(dbExecutor))
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}
}
