package settings

import (
	"fmt"
	"os"

	"github.com/Jguer/yay/v11/pkg/settings/parser"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"golang.org/x/term"
)

func RetrievePacmanConfig(cmdArgs *parser.Arguments, pacmanConfigPath string) (*pacmanconf.Config, bool, error) {
	root := "/"
	if value, _, exists := cmdArgs.GetArg("root", "r"); exists {
		root = value
	}

	pacmanConf, stderr, err := pacmanconf.PacmanConf("--config", pacmanConfigPath, "--root", root)
	if err != nil {
		cmdErr := err
		if stderr != "" {
			cmdErr = fmt.Errorf("%s\n%s", err, stderr)
		}

		return nil, false, cmdErr
	}

	if dbPath, _, exists := cmdArgs.GetArg("dbpath", "b"); exists {
		pacmanConf.DBPath = dbPath
	}

	if arch := cmdArgs.GetArgs("arch"); arch != nil {
		pacmanConf.Architecture = append(pacmanConf.Architecture, arch...)
	}

	if ignoreArray := cmdArgs.GetArgs("ignore"); ignoreArray != nil {
		pacmanConf.IgnorePkg = append(pacmanConf.IgnorePkg, ignoreArray...)
	}

	if ignoreGroupsArray := cmdArgs.GetArgs("ignoregroup"); ignoreGroupsArray != nil {
		pacmanConf.IgnoreGroup = append(pacmanConf.IgnoreGroup, ignoreGroupsArray...)
	}

	if cacheArray := cmdArgs.GetArgs("cachedir"); cacheArray != nil {
		pacmanConf.CacheDir = cacheArray
	}

	if gpgDir, _, exists := cmdArgs.GetArg("gpgdir"); exists {
		pacmanConf.GPGDir = gpgDir
	}

	useColor := pacmanConf.Color && term.IsTerminal(int(os.Stdout.Fd()))

	switch value, _, _ := cmdArgs.GetArg("color"); value {
	case "always":
		useColor = true
	case "auto":
		useColor = term.IsTerminal(int(os.Stdout.Fd()))
	case "never":
		useColor = false
	}

	return pacmanConf, useColor, nil
}
