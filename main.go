package main // import "github.com/Jguer/yay"

import (
	"cmp"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/db/ialpm"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/lua"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

var (
	yayVersion = "13.0.0"            // To be set by compiler.
	localePath = "/usr/share/locale" // To be set by compiler.
)

func initGotext() {
	if envLocalePath := os.Getenv("LOCALE_PATH"); envLocalePath != "" {
		localePath = envLocalePath
	}

	if lc := os.Getenv("LANGUAGE"); lc != "" {
		// Split LANGUAGE by ':' and prioritize the first locale
		// Should fix in gotext to support this
		locales := strings.Split(lc, ":")
		if len(locales) > 0 && locales[0] != "" {
			gotext.Configure(localePath, locales[0], "yay")
		}
	} else {
		gotext.Configure(localePath, cmp.Or(os.Getenv("LC_ALL"), os.Getenv("LC_MESSAGES"), os.Getenv("LANG")), "yay")
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
			fallbackLog.Errorln("Panic occurred:", rec)
			fallbackLog.Errorln("Stack trace:", string(debug.Stack()))
			ret = 1
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

	var luaEngine *lua.Engine
	if luaPath := settings.GetLuaConfigPath(cfg.Debug); luaPath != "" {
		luaEngine, err = lua.Load(fallbackLog, luaPath, cfg)
		if err != nil {
			fallbackLog.Errorln(err)
			ret = 1

			return
		}
		defer luaEngine.Close()
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

	if luaEngine != nil {
		luaEngine.SetLogger(run.Logger.Child("lua"))
	}
	run.Lua = luaEngine
	run.QueryBuilder.SetLua(luaEngine)

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
			fallbackLog.Errorln("Panic occurred in DB operation:", rec)
			fallbackLog.Errorln("Stack trace:", string(debug.Stack()))
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
