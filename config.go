package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// Editor returns the preferred system editor.
func editor() (editor string, args []string) {
	switch {
	case config.Editor != "":
		editor, err := exec.LookPath(config.Editor)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, strings.Fields(config.EditorFlags)
		}

		fallthrough
	case os.Getenv("EDITOR") != "":
		if editorArgs := strings.Fields(os.Getenv("EDITOR")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	case os.Getenv("VISUAL") != "":
		if editorArgs := strings.Fields(os.Getenv("VISUAL")); len(editorArgs) != 0 {
			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				return editor, editorArgs[1:]
			}
		}

		fallthrough
	default:
		fmt.Fprintln(os.Stderr)
		text.Errorln(gotext.Get("%s is not set", text.Bold(text.Cyan("$EDITOR"))))
		text.Warnln(gotext.Get("Add %s or %s to your environment variables", text.Bold(text.Cyan("$EDITOR")), text.Bold(text.Cyan("$VISUAL"))))

		for {
			text.Infoln(gotext.Get("Edit PKGBUILD with?"))

			editorInput, err := getInput("")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			editorArgs := strings.Fields(editorInput)
			if len(editorArgs) == 0 {
				continue
			}

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			return editor, editorArgs[1:]
		}
	}
}

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
