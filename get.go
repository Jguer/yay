package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/db"
	"github.com/Jguer/yay/v10/pkg/download"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

// yay -Gp
func printPkgbuilds(dbExecutor db.Executor, httpClient *http.Client, targets []string) error {
	pkgbuilds, err := download.GetPkgbuilds(dbExecutor, httpClient, targets, config.Runtime.Mode)
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

// yay -G
func getPkgbuilds(dbExecutor db.Executor, config *settings.Configuration, targets []string,
	force bool) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	cloned, errD := download.PKGBUILDRepos(dbExecutor, config.Runtime.CmdRunner,
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
