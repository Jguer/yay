package main // import "github.com/Jguer/yay"

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/db/ialpm"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
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
	var (
		err error
		ctx = context.Background()
		ret = 0
	)

	defer func() {
		if rec := recover(); rec != nil {
			text.Errorln(rec)
			debug.PrintStack()
		}

		os.Exit(ret)
	}()

	initGotext()

	if os.Geteuid() == 0 {
		text.Warnln(gotext.Get("Avoid running yay as root/sudo."))
	}

	config, err = settings.NewConfig(yayVersion)
	if err != nil {
		if str := err.Error(); str != "" {
			text.Errorln(str)
		}

		ret = 1

		return
	}

	if config.Debug {
		text.DebugMode = true
	}

	if errS := config.RunMigrations(
		settings.DefaultMigrations(), config.Runtime.ConfigPath); errS != nil {
		text.Errorln(errS)
	}

	cmdArgs := parser.MakeArguments()

	if err = config.ParseCommandLine(cmdArgs); err != nil {
		if str := err.Error(); str != "" {
			text.Errorln(str)
		}

		ret = 1

		return
	}

	if config.Runtime.SaveConfig {
		if errS := config.Save(config.Runtime.ConfigPath); errS != nil {
			text.Errorln(errS)
		}
	}

	if config.SeparateSources {
		config.Runtime.QueryBuilder = query.NewSourceQueryBuilder(
			config.Runtime.AURClient, config.Runtime.AURCache,
			config.SortBy,
			config.Runtime.Mode, config.SearchBy, config.BottomUp,
			config.SingleLineResults, config.NewInstallEngine)
	} else {
		config.Runtime.QueryBuilder = query.NewMixedSourceQueryBuilder(
			config.Runtime.AURClient, config.Runtime.AURCache, config.SortBy,
			config.Runtime.Mode, config.SearchBy,
			config.BottomUp, config.SingleLineResults, config.NewInstallEngine)
	}

	var useColor bool

	config.Runtime.PacmanConf, useColor, err = settings.RetrievePacmanConfig(cmdArgs, config.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			text.Errorln(str)
		}

		ret = 1

		return
	}

	config.Runtime.CmdBuilder.SetPacmanDBPath(config.Runtime.PacmanConf.DBPath)

	text.UseColor = useColor

	dbExecutor, err := ialpm.NewExecutor(config.Runtime.PacmanConf)
	if err != nil {
		if str := err.Error(); str != "" {
			text.Errorln(str)
		}

		ret = 1

		return
	}

	defer func() {
		if rec := recover(); rec != nil {
			text.Errorln(rec)
			debug.PrintStack()
		}

		dbExecutor.Cleanup()
	}()

	if err = handleCmd(ctx, config, cmdArgs, db.Executor(dbExecutor)); err != nil {
		if str := err.Error(); str != "" {
			text.Errorln(str)
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
