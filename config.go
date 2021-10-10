package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/text"
)

// Verbosity settings for search.
const (
	numberMenu = iota
	detailed
	minimal
)

var yayVersion = "11.0.1"

var localePath = "/usr/share/locale"

// YayConf holds the current config values for yay.
var config *settings.Configuration

func getInput(defaultValue string) (string, error) {
	text.Info()

	if defaultValue != "" || settings.NoConfirm {
		fmt.Println(defaultValue)
		return defaultValue, nil
	}

	reader := bufio.NewReader(os.Stdin)

	buf, overflow, err := reader.ReadLine()
	if err != nil {
		return "", err
	}

	if overflow {
		return "", fmt.Errorf(gotext.Get("input too long"))
	}

	return string(buf), nil
}
