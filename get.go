package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/Jguer/aur"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

// yay -Gp.
func printPkgbuilds(ctx context.Context, dbExecutor download.DBSearcher, aurClient aur.QueryClient,
	httpClient *http.Client, logger *text.Logger, targets []string,
	mode parser.TargetMode, aurURL, pkgbuildPager string,
) error {
	pkgbuilds, err := download.PKGBUILDs(dbExecutor, aurClient, httpClient, logger, targets, aurURL, mode)
	if err != nil {
		logger.Errorln(err)
	}

	output := &strings.Builder{}
	for target, pkgbuild := range pkgbuilds {
		fmt.Fprintf(output, "\n\n# %s\n\n%s", target, string(pkgbuild))
	}

	if err := printPkgbuildOutput(ctx, logger, output.String(), pkgbuildPager); err != nil {
		return err
	}

	if len(pkgbuilds) != len(targets) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := pkgbuilds[target]; !ok {
				missing = append(missing, target)
			}
		}

		logger.Warnln(gotext.Get("Unable to find the following packages:"), " ", strings.Join(missing, ", "))

		return fmt.Errorf("")
	}

	return nil
}

// printPkgbuildOutput writes the collected PKGBUILDs to stdout, or pipes them
// through the configured pager. Unlike the diff pager there is no fallback: an
// unset pkgbuildPager keeps -Gp's plain, script-friendly stdout.
// The pager command is user-controlled (config / Lua), same model as $EDITOR.
func printPkgbuildOutput(ctx context.Context, logger *text.Logger, output, pkgbuildPager string) error {
	if output == "" {
		return nil
	}

	if strings.TrimSpace(pkgbuildPager) == "" {
		logger.Print(output)

		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", pkgbuildPager)
	cmd.Stdin = strings.NewReader(output)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if os.Getenv("LESS") == "" {
		// S: chop long lines; R: raw ANSI; X: no termcap init; F: quit if one screen
		cmd.Env = append(cmd.Env, "LESS=SRXF")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run pkgbuild pager: %w", err)
	}

	return nil
}

// yay -G.
func getPkgbuilds(ctx context.Context, dbExecutor download.DBSearcher, aurClient aur.QueryClient,
	run *runtime.Runtime, targets []string, force bool,
) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cloned, errD := download.PKGBUILDRepos(ctx, dbExecutor, aurClient,
		run.CmdBuilder, run.Logger, targets, run.Cfg.Mode, run.Cfg.AURURL, wd, force)
	if errD != nil {
		run.Logger.Errorln(errD)
	}

	if len(targets) != len(cloned) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := cloned[target]; !ok {
				missing = append(missing, target)
			}
		}

		run.Logger.Warnln(gotext.Get("Unable to find the following packages:"), " ", strings.Join(missing, ", "))

		err = fmt.Errorf("")
	}

	return err
}
