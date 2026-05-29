package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Jguer/aur"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
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

	output := &bytes.Buffer{}
	for target, pkgbuild := range pkgbuilds {
		fmt.Fprintf(output, "\n\n# %s\n\n%s", target, string(pkgbuild))
	}

	if err := printPkgbuildOutput(ctx, logger, output.Bytes(), pkgbuildPager); err != nil {
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

func printPkgbuildOutput(ctx context.Context, logger *text.Logger, output []byte, pkgbuildPager string) error {
	if len(output) == 0 {
		return nil
	}

	if strings.TrimSpace(pkgbuildPager) == "" {
		logger.Print(string(output))
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", pkgbuildPager)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open pkgbuild pager stdin: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run pkgbuild pager: %w", err)
	}

	// Let users quit pagers early without treating the resulting broken pipe as
	// a yay error; the pager's exit status is still checked below.
	_, _ = stdin.Write(output)
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
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
