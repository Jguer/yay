package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	alpm "github.com/jguer/go-alpm"
)

func initPaths() {
	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		if info, err := os.Stat(configHome); err == nil && info.IsDir() {
			configHome = filepath.Join(configHome, "yay")
		} else {
			configHome = filepath.Join(os.Getenv("HOME"), ".config/yay")
		}
	} else {
		configHome = filepath.Join(os.Getenv("HOME"), ".config/yay")
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		if info, err := os.Stat(cacheHome); err == nil && info.IsDir() {
			cacheHome = filepath.Join(cacheHome, "yay")
		} else {
			cacheHome = filepath.Join(os.Getenv("HOME"), ".cache/yay")
		}
	} else {
		cacheHome = filepath.Join(os.Getenv("HOME"), ".cache/yay")
	}

	configFile = filepath.Join(configHome, configFileName)
	vcsFile = filepath.Join(cacheHome, vcsFileName)
}

func initConfig() (err error) {
	defaultSettings(&config)

	if _, err = os.Stat(configFile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(configFile), 0755)
		if err != nil {
			err = fmt.Errorf("Unable to create config directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
			return
		}
		// Save the default config if nothing is found
		config.saveConfig()
	} else {
		cfile, errf := os.OpenFile(configFile, os.O_RDWR|os.O_CREATE, 0644)
		if errf != nil {
			fmt.Printf("Error reading config: %s\n", err)
		} else {
			defer cfile.Close()
			decoder := json.NewDecoder(cfile)
			err = decoder.Decode(&config)
			if err != nil {
				fmt.Println("Loading default Settings.\nError reading config:",
					err)
				defaultSettings(&config)
			}
			if _, err = os.Stat(config.BuildDir); os.IsNotExist(err) {
				err = os.MkdirAll(config.BuildDir, 0755)
				if err != nil {
					err = fmt.Errorf("Unable to create BuildDir directory:\n%s\n"+
						"The error was:\n%s", config.BuildDir, err)
					return
				}
			}
		}
	}

	return
}

func initVCS() (err error) {
	if _, err = os.Stat(vcsFile); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(vcsFile), 0755)
		if err != nil {
			err = fmt.Errorf("Unable to create vcs directory:\n%s\n"+
				"The error was:\n%s", filepath.Dir(configFile), err)
			return
		}
	} else {
		vfile, err := os.OpenFile(vcsFile, os.O_RDONLY|os.O_CREATE, 0644)
		if err == nil {
			defer vfile.Close()
			decoder := json.NewDecoder(vfile)
			_ = decoder.Decode(&savedInfo)
		}
	}

	return
}

func initAlpm() (err error) {
	var value string
	var exists bool
	//var double bool

	value, _, exists = cmdArgs.getArg("config")
	if exists {
		config.PacmanConf = value
	}

	alpmConf, err = readAlpmConfig(config.PacmanConf)
	if err != nil {
		err = fmt.Errorf("Unable to read Pacman conf: %s", err)
		return
	}

	value, _, exists = cmdArgs.getArg("dbpath", "b")
	if exists {
		alpmConf.DBPath = value
	}

	value, _, exists = cmdArgs.getArg("root", "r")
	if exists {
		alpmConf.RootDir = value
	}

	value, _, exists = cmdArgs.getArg("arch")
	if exists {
		alpmConf.Architecture = value
	}

	value, _, exists = cmdArgs.getArg("ignore")
	if exists {
		alpmConf.IgnorePkg = append(alpmConf.IgnorePkg, strings.Split(value, ",")...)
	}

	value, _, exists = cmdArgs.getArg("ignoregroup")
	if exists {
		alpmConf.IgnoreGroup = append(alpmConf.IgnoreGroup, strings.Split(value, ",")...)
	}

	//TODO
	//current system does not allow duplicate arguments
	//but pacman allows multiple cachdirs to be passed
	//for now only handle one cache dir
	value, _, exists = cmdArgs.getArg("cachdir")
	if exists {
		alpmConf.CacheDir = []string{value}
	}

	value, _, exists = cmdArgs.getArg("gpgdir")
	if exists {
		alpmConf.GPGDir = value
	}

	alpmHandle, err = alpmConf.CreateHandle()
	if err != nil {
		err = fmt.Errorf("Unable to CreateHandle: %s", err)
		return
	}

	value, _, _ = cmdArgs.getArg("color")
	if value == "always" || value == "auto" {
		useColor = true
	} else if value == "never" {
		useColor = false
	} else {
		useColor = alpmConf.Options&alpm.ConfColor > 0
	}

	alpmHandle.SetQuestionCallback(questionCallback)

	return
}

func main() {
	var status int
	var err error

	if 0 == os.Geteuid() {
		fmt.Println("Please avoid running yay as root/sudo.")
	}

	err = cmdArgs.parseCommandLine()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	initPaths()

	err = initConfig()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	err = initVCS()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup

	}

	err = initAlpm()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	err = handleCmd()
	if err != nil {
		if err.Error() != "" {
			fmt.Println(err)
		}

		status = 1
		goto cleanup
	}

cleanup:
	//cleanup
	//from here on out don't exit if an error occurs
	//if we fail to save the configuration
	//at least continue on and try clean up other parts

	if alpmHandle != nil {
		err = alpmHandle.Release()
		if err != nil {
			fmt.Println(err)
			status = 1
		}
	}

	os.Exit(status)
}
