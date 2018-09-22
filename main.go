package main // import "github.com/Jguer/yay"

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	alpm "github.com/jguer/go-alpm"
)

func setPaths() error {
	if config.home = os.Getenv("XDG_CONFIG_HOME"); config.home != "" {
		config.home = filepath.Join(config.home, "yay")
	} else if config.home = os.Getenv("HOME"); config.home != "" {
		config.home = filepath.Join(config.home, ".config/yay")
	} else {
		return fmt.Errorf("XDG_CONFIG_HOME and HOME unset")
	}

	if config.cacheHome = os.Getenv("XDG_CACHE_HOME"); config.cacheHome != "" {
		config.cacheHome = filepath.Join(config.cacheHome, "yay")
	} else if config.cacheHome = os.Getenv("HOME"); config.cacheHome != "" {
		config.cacheHome = filepath.Join(config.cacheHome, ".cache/yay")
	} else {
		return fmt.Errorf("XDG_CACHE_HOME and HOME unset")
	}

	config.file = filepath.Join(config.home, configFileName)
	config.vcsFile = filepath.Join(config.cacheHome, vcsFileName)

	return nil
}

func initVCS() error {
	vfile, err := os.Open(config.vcsFile)
	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("Failed to open vcs file '%s': %s", config.vcsFile, err)
	}

	defer vfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&config.savedInfo); err != nil {
			return fmt.Errorf("Failed to read vcs '%s': %s", config.vcsFile, err)
		}
	}

	return nil
}

func initHomeDirs() error {
	if _, err := os.Stat(config.home); os.IsNotExist(err) {
		if err = os.MkdirAll(config.home, 0755); err != nil {
			return fmt.Errorf("Failed to create config directory '%s': %s", config.home, err)
		}
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(config.cacheHome); os.IsNotExist(err) {
		if err = os.MkdirAll(config.cacheHome, 0755); err != nil {
			return fmt.Errorf("Failed to create cache directory '%s': %s", config.cacheHome, err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initbuilddir() error {
	if _, err := os.Stat(config.value["builddir"]); os.IsNotExist(err) {
		if err = os.MkdirAll(config.value["builddir"], 0755); err != nil {
			return fmt.Errorf("Failed to create builddir directory '%s': %s", config.value["builddir"], err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initAlpm() error {
	var err error
	var stderr string

	root := "/"
	if value, _, exists := cmdArgs.getArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err = pacmanconf.PacmanConf("--config", config.value["pacmanconf"], "--root", root)
	if err != nil {
		return fmt.Errorf("%s", stderr)
	}

	if value, _, exists := cmdArgs.getArg("dbpath", "b"); exists {
		pacmanConf.DBPath = value
	}

	if value, _, exists := cmdArgs.getArg("arch"); exists {
		pacmanConf.Architecture = value
	}

	if value, _, exists := cmdArgs.getArg("ignore"); exists {
		pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, strings.Split(value, ",")...)
	}

	if value, _, exists := cmdArgs.getArg("ignoregroup"); exists {
		pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	//TODO
	//current system does not allow duplicate arguments
	//but pacman allows multiple cachdirs to be passed
	//for now only handle one cache dir
	if value, _, exists := cmdArgs.getArg("cachdir"); exists {
		pacmanConf.CacheDir = []string{value}
	}

	if value, _, exists := cmdArgs.getArg("gpgdir"); exists {
		pacmanConf.GPGDir = value
	}

	if err = initAlpmHandle(); err != nil {
		return err
	}

	if value, _, _ := cmdArgs.getArg("color"); value == "always" {
		config.useColor = true
	} else if value == "auto" {
		config.useColor = isTty()
	} else if value == "never" {
		config.useColor = false
	} else {
		config.useColor = pacmanConf.Color && isTty()
	}

	return nil
}

func initAlpmHandle() error {
	var err error

	if alpmHandle != nil {
		if err := alpmHandle.Release(); err != nil {
			return err
		}
	}

	if alpmHandle, err = alpm.Init(pacmanConf.RootDir, pacmanConf.DBPath); err != nil {
		return fmt.Errorf("Unable to CreateHandle: %s", err)
	}

	if err = configureAlpm(pacmanConf); err != nil {
		return err
	}

	alpmHandle.SetQuestionCallback(questionCallback)
	alpmHandle.SetLogCallback(logCallback)
	return nil
}

func exitOnError(err error) {
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Println(str)
		}
		cleanup()
		os.Exit(1)
	}
}

func cleanup() int {
	if alpmHandle != nil {
		if err := alpmHandle.Release(); err != nil {
			fmt.Println(err)
			return 1
		}
	}

	return 0
}

func main() {
	if 0 == os.Geteuid() {
		fmt.Println("Please avoid running yay as root/sudo.")
	}

	exitOnError(setPaths())
	config.defaultSettings()
	exitOnError(initHomeDirs())
	exitOnError(initConfig())
	exitOnError(cmdArgs.parseCommandLine())
	// if config.shouldSaveConfig {
	// 	config.saveConfig()
	// }
	config.expandEnv()
	exitOnError(initbuilddir())
	exitOnError(initVCS())
	exitOnError(initAlpm())
	exitOnError(handleCmd())
	os.Exit(cleanup())
}
