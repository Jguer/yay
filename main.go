package main // import "github.com/Jguer/yay"

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"
	"golang.org/x/term"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/db/ialpm"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	if os.Getenv("LC_MESSAGES") != "" {
		gotext.Configure(localePath, os.Getenv("LC_MESSAGES"), "yay")
	} else {
		gotext.Configure(localePath, os.Getenv("LANG"), "yay")
	}
}

func initAlpm(cmdArgs *settings.Arguments, pacmanConfigPath string) (*pacmanconf.Config, bool, error) {
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

func main() {
	var err error
	ret := 0
	defer func() { os.Exit(ret) }()
	initGotext()
	if os.Geteuid() == 0 {
		text.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	config, err = settings.NewConfig()
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	cmdArgs := settings.MakeArguments()
	err = cmdArgs.ParseCommandLine(config)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	if config.Runtime.SaveConfig {
		if errS := config.Save(config.Runtime.ConfigPath); errS != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	var useColor bool
	config.Runtime.PacmanConf, useColor, err = initAlpm(cmdArgs, config.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	text.UseColor = useColor

	dbExecutor, err := ialpm.NewExecutor(config.Runtime.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			fmt.Fprintln(os.Stderr, str)
		}
		ret = 1
		return
	}

	defer dbExecutor.Cleanup()
	err = handleCmd(cmdArgs, db.Executor(dbExecutor))

	if err != nil {
		if str := err.Error(); str != "" && !strings.Contains(str, "exit status") {
			fmt.Fprintln(os.Stderr, str)
		}

		if exitError, ok := err.(*exec.ExitError); ok {
			// mirror pacman exit code when applicable
			ret = exitError.ExitCode()
			return
		}

		// fallback
		ret = 1
		return
	}
}
