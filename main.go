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

var config *settings.YayConfig

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	gotext.Configure(localePath, os.Getenv("LANG"), "yay")
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

func initAlpm(pacmanConfigPath string) (*alpm.Handle, *pacmanconf.Config, error) {
	root := "/"
	if config.Root != "" {
		root = config.Root
	}

	pacmanConf, stderr, err := pacmanconf.PacmanConf("--config", pacmanConfigPath, "--root", root)
	if err != nil {
		return nil, nil, fmt.Errorf("%s", stderr)
	}

	if config.DbPath != "" {
		pacmanConf.DBPath = config.DbPath
	}

	if config.Arch != "" {
		pacmanConf.Architecture = config.Arch
	}

	if config.GpgDir != "" {
		pacmanConf.GPGDir = config.GpgDir
	}

	pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, config.Ignore...)
	pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, config.IgnoreGroup...)
	pacmanConf.CacheDir = append(pacmanConf.IgnoreGroup, config.CacheDir...)

	alpmHandle, err := initAlpmHandle(pacmanConf, nil)
	if err != nil {
		return nil, nil, err
	}

	switch config.Color {
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
		cleanup(config.Alpm)
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

	config = settings.DefaultConfig()
	loaded, err := config.LoadFile(config.ConfigPath)
	exitOnError(err)
	if !loaded {
		_, err = config.LoadFile(settings.SystemConfigFile)
		exitOnError(err)
	}
	exitOnError(settings.ParseCommandLine(config))
	exitOnError(initBuildDir())
	exitOnError(initVCS(config.VCSPath))
	config.Alpm, config.Pacman, err = initAlpm(config.PacmanConf)
	exitOnError(err)

	exitOnError(handleCmd(config.Flags(), config.Alpm))
	os.Exit(cleanup(config.Alpm))
}
