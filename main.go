package main // import "github.com/Jguer/yay"

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/vcs"
)

type (
	Paths struct {
		ConfigBaseDir  string
		ConfigFilePath string
		CacheBaseDir   string
		VCSFilePath    string
	}
)

const (
	// configFileName holds the name of the config file.
	configFileName string = "config.json"
	// vcsFileName holds the name of the vcs file.
	vcsFileName string = "vcs.json"
)

func cleanup(alpmHandle *alpm.Handle) int {
	if alpmHandle != nil {
		if err := alpmHandle.Release(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	return 0
}

func exitOnError(alpmHandle *alpm.Handle, err error) {
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		cleanup(alpmHandle)
		os.Exit(1)
	}
}

func initBuildDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("Failed to create BuildDir directory '%s': %s", dir, err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initConfig(configFile string, config *runtime.Configuration) error {
	cfile, err := os.Open(configFile)
	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("Failed to open config file '%s': %s", configFile, err)
	}

	defer cfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(cfile)
		if err = decoder.Decode(&config); err != nil {
			return fmt.Errorf("Failed to read config '%s': %s", configFile, err)
		}
	}

	return nil
}

func initHomeDirs(configBaseDir string, cacheBaseDir string) error {
	if _, err := os.Stat(configBaseDir); os.IsNotExist(err) {
		if err = os.MkdirAll(configBaseDir, 0755); err != nil {
			return fmt.Errorf("Failed to create config directory '%s': %s", configBaseDir, err)
		}
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(cacheBaseDir); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheBaseDir, 0755); err != nil {
			return fmt.Errorf("Failed to create cache directory '%s': %s", cacheBaseDir, err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func isTty() bool {
	cmd := exec.Command("test", "-t", "1")
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	return err == nil
}

func main() {
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "Please avoid running yay as root/sudo.")
	}

	runpath, err := setPaths()
	exitOnError(nil, err)

	config := runtime.DefaultSettings()

	err = initHomeDirs(runpath.ConfigBaseDir, runpath.CacheBaseDir)
	exitOnError(nil, err)

	err = initConfig(runpath.ConfigFilePath, config)
	exitOnError(nil, err)

	cmdArgs, err := config.ParseCommandLine()
	exitOnError(nil, err)

	if config.ShouldSaveConfig {
		err = config.SaveConfig(runpath.ConfigFilePath)
		if err != nil {
			fmt.Println(err)
		}
	}
	config.ExpandEnv()

	err = initBuildDir(config.BuildDir)
	exitOnError(nil, err)

	savedInfo, err := vcs.ReadVCSFromFile(runpath.VCSFilePath)
	exitOnError(nil, err)

	pacmanConf, err := runtime.InitPacmanConf(cmdArgs, config.PacmanConf)
	exitOnError(nil, err)

	alpmHandle, err := runtime.InitAlpmHandle(config, pacmanConf, nil)
	exitOnError(alpmHandle, err)

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

	err = handleCmd(config, pacmanConf, cmdArgs, alpmHandle, savedInfo)
	exitOnError(alpmHandle, err)

	exitOnError(alpmHandle, err)

	os.Exit(cleanup(alpmHandle))
}

func setPaths() (*Paths, error) {
	runPath := &Paths{}
	if runPath.ConfigBaseDir = os.Getenv("XDG_CONFIG_HOME"); runPath.ConfigBaseDir != "" {
		runPath.ConfigBaseDir = filepath.Join(runPath.ConfigBaseDir, "yay")
	} else if runPath.ConfigBaseDir = os.Getenv("HOME"); runPath.ConfigBaseDir != "" {
		runPath.ConfigBaseDir = filepath.Join(runPath.ConfigBaseDir, ".config/yay")
	} else {
		return nil, fmt.Errorf("XDG_CONFIG_HOME and HOME unset")
	}

	if runPath.CacheBaseDir = os.Getenv("XDG_CACHE_HOME"); runPath.CacheBaseDir != "" {
		runPath.CacheBaseDir = filepath.Join(runPath.CacheBaseDir, "yay")
	} else if runPath.CacheBaseDir = os.Getenv("HOME"); runPath.CacheBaseDir != "" {
		runPath.CacheBaseDir = filepath.Join(runPath.CacheBaseDir, ".cache/yay")
	} else {
		return nil, fmt.Errorf("XDG_CACHE_HOME and HOME unset")
	}

	runPath.ConfigFilePath = filepath.Join(runPath.ConfigBaseDir, configFileName)
	runPath.VCSFilePath = filepath.Join(runPath.CacheBaseDir, vcsFileName)

	return runPath, nil
}
