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
	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config/yay")
	} else {
		return fmt.Errorf("XDG_CONFIG_HOME and HOME unset")
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, "yay")
	} else if cacheHome = os.Getenv("HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, ".cache/yay")
	} else {
		return fmt.Errorf("XDG_CACHE_HOME and HOME unset")
	}

	configFile = filepath.Join(configHome, configFileName)
	vcsFile = filepath.Join(cacheHome, vcsFileName)

	return nil
}

func initConfig() error {
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

func initVCS() error {
	vfile, err := os.Open(vcsFile)
	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("Failed to open vcs file '%s': %s", vcsFile, err)
	}

	defer vfile.Close()
	if !os.IsNotExist(err) {
		decoder := json.NewDecoder(vfile)
		if err = decoder.Decode(&savedInfo); err != nil {
			return fmt.Errorf("Failed to read vcs '%s': %s", vcsFile, err)
		}
	}

	return nil
}

func initHomeDirs() error {
	if _, err := os.Stat(configHome); os.IsNotExist(err) {
		if err = os.MkdirAll(configHome, 0755); err != nil {
			return fmt.Errorf("Failed to create config directory '%s': %s", configHome, err)
		}
	} else if err != nil {
		return err
	}

	if _, err := os.Stat(cacheHome); os.IsNotExist(err) {
		if err = os.MkdirAll(cacheHome, 0755); err != nil {
			return fmt.Errorf("Failed to create cache directory '%s': %s", cacheHome, err)
		}
	} else if err != nil {
		return err
	}

	return nil
}

func initBuildDir() error {
	if _, err := os.Stat(config.BuildDir); os.IsNotExist(err) {
		if err = os.MkdirAll(config.BuildDir, 0755); err != nil {
			return fmt.Errorf("Failed to create BuildDir directory '%s': %s", config.BuildDir, err)
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

	pacmanConf, stderr, err = pacmanconf.PacmanConf("--config", config.PacmanConf, "--root", root)
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

	if value, _, _ := cmdArgs.getArg("color"); value == "always" || value == "auto" {
		useColor = true
	} else if value == "never" {
		useColor = false
	} else {
		useColor = pacmanConf.Color
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
	if shouldSaveConfig {
		config.saveConfig()
	}
	config.expandEnv()
	exitOnError(initBuildDir())
	exitOnError(initVCS())
	exitOnError(initAlpm())
	exitOnError(handleCmd())
	os.Exit(cleanup())
}
