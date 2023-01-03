package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
)

// yay -Gp.
func printPkgbuilds(dbExecutor download.DBSearcher, httpClient *http.Client, targets []string,
	mode parser.TargetMode, aurURL string,
) error {
	pkgbuilds, err := download.PKGBUILDs(dbExecutor, httpClient, targets, aurURL, mode)
	if err != nil {
		text.Errorln(err)
	}

	if len(pkgbuilds) != 0 {
		for target, pkgbuild := range pkgbuilds {
			fmt.Printf("\n\n# %s\n\n", target)
			fmt.Print(string(pkgbuild))
		}
	}

	if len(pkgbuilds) != len(targets) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := pkgbuilds[target]; !ok {
				missing = append(missing, target)
			}
		}

		text.Warnln(gotext.Get("Unable to find the following packages:"), strings.Join(missing, ", "))

		return fmt.Errorf("")
	}

	return nil
}

// yay -G.
func getPkgbuilds(ctx context.Context, dbExecutor download.DBSearcher,
	config *settings.Configuration, targets []string, force bool,
) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cloned, errD := download.PKGBUILDRepos(ctx, dbExecutor,
		config.Runtime.CmdBuilder, targets, config.Runtime.Mode, config.AURURL, wd, force)
	if errD != nil {
		text.Errorln(errD)
	}

	if len(targets) != len(cloned) {
		missing := []string{}

		for _, target := range targets {
			if _, ok := cloned[target]; !ok {
				missing = append(missing, target)
			}
		}

		text.Warnln(gotext.Get("Unable to find the following packages:"), strings.Join(missing, ", "))

		err = fmt.Errorf("")
	}

	return err
}
