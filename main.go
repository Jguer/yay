package main // import "github.com/Jguer/yay"

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/customrepo"
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
		// Split LANGUAGE by ':' and prioritize the first locale
		// Should fix in gotext to support this
		locales := strings.Split(lc, ":")
		if len(locales) > 0 && locales[0] != "" {
			gotext.Configure(localePath, locales[0], "yay")
		}
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

	// Handle custom repository commands before initializing runtime
	if cmdArgs.ExistsArg("repo-add") {
		handleRepoAddEarly(ctx, cfg, cmdArgs, fallbackLog)
		return
	}
	if cmdArgs.ExistsArg("repo-remove") {
		handleRepoRemoveEarly(ctx, cfg, cmdArgs, fallbackLog)
		return
	}
	if cmdArgs.ExistsArg("repo-list") {
		handleRepoListEarly(ctx, cfg, cmdArgs, fallbackLog)
		return
	}
	if cmdArgs.ExistsArg("repo-update") {
		handleRepoUpdateEarly(ctx, cfg, cmdArgs, fallbackLog)
		return
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

// handleRepoAddEarly handles --repo-add without initializing runtime
func handleRepoAddEarly(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, logger *text.Logger) {
	// Get arguments from the repo-add option
	repoAddArgs := cmdArgs.Options["repo-add"]
	if repoAddArgs == nil || len(repoAddArgs.Args) < 3 {
		logger.Errorln("Usage: --repo-add <name> <type> <url/path>")
		return
	}

	name := repoAddArgs.Args[0]
	repoType := repoAddArgs.Args[1]
	urlOrPath := repoAddArgs.Args[2]

	// Create new custom repository
	repo := settings.CustomRepo{
		Name:       name,
		Type:       repoType,
		Searchable: true,
		Priority:   100, // Default priority
	}

	if repoType == "local" {
		repo.Path = urlOrPath
	} else {
		repo.URL = urlOrPath
	}

	// Add to configuration
	cfg.CustomRepos = append(cfg.CustomRepos, repo)

	// Save configuration
	configPath := settings.GetConfigPath()
	if err := cfg.Save(configPath, yayVersion); err != nil {
		logger.Errorln("Failed to save configuration:", err)
		return
	}

	logger.Printf("Added custom repository: %s (%s)\n", name, repoType)
}

// handleRepoRemoveEarly handles --repo-remove without initializing runtime
func handleRepoRemoveEarly(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, logger *text.Logger) {
	// Get arguments from the repo-remove option
	repoRemoveArgs := cmdArgs.Options["repo-remove"]
	if repoRemoveArgs == nil || len(repoRemoveArgs.Args) < 1 {
		logger.Errorln("Usage: --repo-remove <name>")
		return
	}

	name := repoRemoveArgs.Args[0]

	// Find and remove repository
	for i, repo := range cfg.CustomRepos {
		if repo.Name == name {
			cfg.CustomRepos = append(cfg.CustomRepos[:i], cfg.CustomRepos[i+1:]...)
			
			// Save configuration
			configPath := settings.GetConfigPath()
			if err := cfg.Save(configPath, yayVersion); err != nil {
				logger.Errorln("Failed to save configuration:", err)
				return
			}

			logger.Printf("Removed custom repository: %s\n", name)
			return
		}
	}

	logger.Errorln("Repository not found:", name)
}

// handleRepoListEarly handles --repo-list without initializing runtime
func handleRepoListEarly(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, logger *text.Logger) {
	if len(cfg.CustomRepos) == 0 {
		logger.Println("No custom repositories configured.")
		return
	}

	logger.Println(text.Bold("Custom Repositories:"))
	logger.Println()

	for _, repo := range cfg.CustomRepos {
		logger.Printf("  %s (%s)\n", text.Bold(repo.Name), repo.Type)
		if repo.Type == "local" {
			logger.Printf("    Path: %s\n", repo.Path)
		} else {
			logger.Printf("    URL: %s\n", repo.URL)
		}
		logger.Printf("    Searchable: %t\n", repo.Searchable)
		logger.Printf("    Priority: %d\n", repo.Priority)
		logger.Println()
	}
}

// handleRepoUpdateEarly handles --repo-update without initializing runtime
func handleRepoUpdateEarly(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments, logger *text.Logger) {
	// Create custom repository manager
	cacheDir, err := customrepo.GetDefaultCacheDir()
	if err != nil {
		logger.Warnln("Failed to get cache directory for custom repositories:", err)
		cacheDir = "/tmp/yay"
	}
	
	factory := customrepo.NewRepositoryFactory(cacheDir)
	customRepoMgr, err := factory.CreateManagerFromConfig(cfg)
	if err != nil {
		logger.Errorln("Failed to create custom repository manager:", err)
		return
	}
	
	logger.Println("Updating custom repositories...")
	
	if err := customRepoMgr.UpdateAll(ctx); err != nil {
		logger.Warnln("Some repositories failed to update:", err)
	} else {
		logger.Println(text.Bold(text.Green("All custom repositories updated successfully.")))
	}
}
