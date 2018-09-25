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
		return nil
	} else if _, ok := y.value[key]; ok {
		y.value[key] = value
		return nil
	}

	return fmt.Errorf("%s does not belong in the option section", key)

}

func (y *yayConfig) setIntOption(key string, value string) error {
	if _, ok := y.num[key]; ok {
		tmp, err := strconv.Atoi(value)
		if err == nil {
			y.num[key] = tmp
			return nil
		}
		return err
	}

	return fmt.Errorf("%s does not belong in the intoption section", key)
}

func initConfig() error {
	file := filepath.Join(config.home, configFileName)

	if _, err := os.Stat(file); os.IsNotExist(err) {
		if _, err := os.Stat("/etc/yay.conf"); !os.IsNotExist(err) {
			file = "/etc/yay.conf"
		} else {
			return nil
		}
	}

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
