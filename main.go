package main // import "github.com/Jguer/yay"

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db/ialpm"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

var (
	yayVersion = "12.0.4"            // To be set by compiler.
	localePath = "/usr/share/locale" // To be set by compiler.
)

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	if lc := os.Getenv("LANGUAGE"); lc != "" {
		gotext.Configure(localePath, lc, "yay")
	} else if lc := os.Getenv("LC_ALL"); lc != "" {
		gotext.Configure(localePath, lc, "yay")
	} else if lc := os.Getenv("LC_MESSAGES"); lc != "" {
		gotext.Configure(localePath, lc, "yay")
	} else {
		gotext.Configure(localePath, os.Getenv("LANG"), "yay")
	}
}

func main() {
	fallbackLog := text.NewLogger(os.Stdout, os.Stderr, os.Stdin, false, "fallback")
	var (
		err error
		ctx = context.Background()
		ret = 0
	)

	defer func() {
		if rec := recover(); rec != nil {
			fallbackLog.Errorln(rec)
			debug.PrintStack()
		}

		os.Exit(ret)
	}()

	initGotext()

	if os.Geteuid() == 0 {
		fallbackLog.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	configPath := settings.GetConfigPath()
	// Parse config
	cfg, err := settings.NewConfig(fallbackLog, configPath, yayVersion)
	if err != nil {
		if str := err.Error(); str != "" {
			fallbackLog.Errorln(str)
		}

		ret = 1

		return
	}

	if errS := cfg.RunMigrations(fallbackLog,
		settings.DefaultMigrations(), configPath, yayVersion); errS != nil {
		fallbackLog.Errorln(errS)
	}

	cmdArgs := parser.MakeArguments()

	// Parse command line
	if err = cfg.ParseCommandLine(cmdArgs); err != nil {
		if str := err.Error(); str != "" {
			fallbackLog.Errorln(str)
		}

		ret = 1

		return
	}

	if cfg.SaveConfig {
		if errS := cfg.Save(configPath, yayVersion); errS != nil {
			fallbackLog.Errorln(errS)
		}
	}

	// Build run
	run, err := runtime.NewRuntime(cfg, cmdArgs, yayVersion)
	if err != nil {
		if str := err.Error(); str != "" {
			fallbackLog.Errorln(str)
		}

		ret = 1

		return
	}

	dbExecutor, err := ialpm.NewExecutor(run.PacmanConf, run.Logger.Child("db"))
	if err != nil {
		if str := err.Error(); str != "" {
			fallbackLog.Errorln(str)
		}

		ret = 1

		return
	}

	defer func() {
		if rec := recover(); rec != nil {
			fallbackLog.Errorln(rec, string(debug.Stack()))
		}

		dbExecutor.Cleanup()
	}()

	if err = handleCmd(ctx, run, cmdArgs, dbExecutor); err != nil {
		if str := err.Error(); str != "" {
			fallbackLog.Errorln(str)
		}

		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			// mirror pacman exit code when applicable
			ret = exitError.ExitCode()
			return
		}

		// fallback
		ret = 1
	}
}
