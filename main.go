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

	err = initAlpmHandle()
	if err != nil {
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

	return
}

func initAlpmHandle() (err error) {
	if alpmHandle != nil {
		err = alpmHandle.Release()
		if err != nil {
			return err
		}
	}

	alpmHandle, err = alpmConf.CreateHandle()
	if err != nil {
		err = fmt.Errorf("Unable to CreateHandle: %s", err)
		return
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

	err = setPaths()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	err = initConfig()
	if err != nil {
		fmt.Println(err)
		status = 1
		goto cleanup
	}

	cmdArgs.extractYayOptions()

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
