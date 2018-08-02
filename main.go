package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	alpm "github.com/jguer/go-alpm"
)

func setPaths() error {
	if _configHome, set := os.LookupEnv("XDG_CONFIG_HOME"); set {
		if _configHome == "" {
			return fmt.Errorf("XDG_CONFIG_HOME set but empty")
		}
		configHome = filepath.Join(_configHome, "yay")
	} else if _configHome, set := os.LookupEnv("HOME"); set {
		if _configHome == "" {
			return fmt.Errorf("HOME set but empty")
		}
		configHome = filepath.Join(_configHome, ".config/yay")
	} else {
		return fmt.Errorf("XDG_CONFIG_HOME and HOME unset")
	}

	if _cacheHome, set := os.LookupEnv("XDG_CACHE_HOME"); set {
		if _cacheHome == "" {
			return fmt.Errorf("XDG_CACHE_HOME set but empty")
		}
		cacheHome = filepath.Join(_cacheHome, "yay")
	} else if _cacheHome, set := os.LookupEnv("HOME"); set {
		if _cacheHome == "" {
			return fmt.Errorf("XDG_CACHE_HOME set but empty")
		}
		cacheHome = filepath.Join(_cacheHome, ".cache/yay")
	} else {
		return fmt.Errorf("XDG_CACHE_HOME and HOME unset")
	}

	configFile = filepath.Join(configHome, configFileName)
	vcsFile = filepath.Join(cacheHome, vcsFileName)

	return nil
}

func initConfig() error {
	defaultSettings(&config)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
			return fmt.Errorf("Unable to create config directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
		}
		// Save the default config if nothing is found
		return config.saveConfig()
	} else if err != nil {
		return err
	}

	cfile, err := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Error reading config: %s\n", err)
	}

	defer cfile.Close()
	decoder := json.NewDecoder(cfile)
	if err = decoder.Decode(&config); err != nil {
		return fmt.Errorf("Error reading config: %s",
			err)
	}

	if _, err = os.Stat(config.BuildDir); os.IsNotExist(err) {
		if err = os.MkdirAll(config.BuildDir, 0755); err != nil {
			return fmt.Errorf("Unable to create BuildDir directory:\n%s\n"+
				"The error was:\n%s", config.BuildDir, err)
		}
	}

	return err
}

func initVCS() error {
	if _, err := os.Stat(vcsFile); os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(vcsFile), 0755); err != nil {
			return fmt.Errorf("Unable to create vcs directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
		}
	} else if err != nil {
		return err
	}

	vfile, err := os.OpenFile(vcsFile, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	defer vfile.Close()
	decoder := json.NewDecoder(vfile)
	_ = decoder.Decode(&savedInfo)

	return nil
}

func initAlpm() error {
	var err error

	if alpmConf, err = readAlpmConfig(config.PacmanConf); err != nil {
		return fmt.Errorf("Unable to read Pacman conf: %s", err)
	}

	if value, _, exists := cmdArgs.getArg("dbpath", "b"); exists {
		alpmConf.DBPath = value
	}

	if value, _, exists := cmdArgs.getArg("root", "r"); exists {
		alpmConf.RootDir = value
	}

	if value, _, exists := cmdArgs.getArg("arch"); exists {
		alpmConf.Architecture = value
	}

	if value, _, exists := cmdArgs.getArg("ignore"); exists {
		alpmConf.IgnorePkg = append(alpmConf.IgnorePkg, strings.Split(value, ",")...)
	}

	if value, _, exists := cmdArgs.getArg("ignoregroup"); exists {
		alpmConf.IgnoreGroup = append(alpmConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	//TODO
	//current system does not allow duplicate arguments
	//but pacman allows multiple cachdirs to be passed
	//for now only handle one cache dir
	if value, _, exists := cmdArgs.getArg("cachdir"); exists {
		alpmConf.CacheDir = []string{value}
	}

	if value, _, exists := cmdArgs.getArg("gpgdir"); exists {
		alpmConf.GPGDir = value
	}

	if err = initAlpmHandle(); err != nil {
		return err
	}

	if value, _, _ := cmdArgs.getArg("color"); value == "always" || value == "auto" {
		useColor = true
	} else if value == "never" {
		useColor = false
	} else {
		useColor = alpmConf.Options&alpm.ConfColor > 0
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

	if alpmHandle, err = alpmConf.CreateHandle(); err != nil {
		return fmt.Errorf("Unable to CreateHandle: %s", err)
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

	exitOnError(cmdArgs.parseCommandLine())
	exitOnError(setPaths())
	exitOnError(initConfig())
	cmdArgs.extractYayOptions()
	exitOnError(initVCS())
	exitOnError(initAlpm())
	exitOnError(handleCmd())
	os.Exit(cleanup())
}
