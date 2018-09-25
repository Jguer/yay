package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Morganamilo/go-pacmanconf/ini"
)

func parseCallback(fileName string, line int, section string,
	key string, value string, data interface{}) (err error) {
	if line < 0 {
		return fmt.Errorf("unable to read file: %s: %s", fileName, section)
	}
	if key == "" && value == "" {
		return nil
	}

	key = strings.ToLower(key)

	if section == "options" {
		err = config.setOption(key, value)
	} else if section == "intoptions" {
		err = config.setIntOption(key, value)
	} else if section == "menus" {
		err = config.setMenus(key, value)
	} else if section == "answer" {
		err = config.setAnswer(key, value)
	} else {
		err = fmt.Errorf("line %d is not in a section: %s", line, fileName)
	}

	return
}

func (y *yayConfig) setMenus(key string, value string) error {
	switch key {
	case "clean", "diff", "edit", "upgrade":
		y.boolean[key+"menu"] = true
		return nil
	}
	return fmt.Errorf("%s does not belong in the answer section", key)
}

func (y *yayConfig) setAnswer(key string, value string) error {
	switch key {
	case "clean", "diff", "edit", "upgrade":
		y.value[key] = value
		return nil
	}

	return fmt.Errorf("%s does not belong in the answer section", key)

}

func (y *yayConfig) setOption(key string, value string) error {
	if _, ok := y.boolean[key]; ok {
		y.boolean[key] = true
	}

	y.value[key] = value
	return nil
}

func (y *yayConfig) setIntOption(key string, value string) error {
	tmp, err := strconv.Atoi(value)
	if err == nil {
		y.num[key] = tmp
	}
	return nil
}

func initConfig() error {
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

	file := filepath.Join(config.home, configFileName)
	config.vcsFile = filepath.Join(config.cacheHome, vcsFileName)

	iniBytes, err := ioutil.ReadFile(file)

	if err != nil {
		return fmt.Errorf("Failed to open config file '%s': %v", file, err)
	}

	// Toggle all switches false
	for k := range config.boolean {
		config.boolean[k] = false
	}

	err = ini.Parse(string(iniBytes), parseCallback, nil)

	return err
}
