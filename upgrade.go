package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/intrange"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/upgrade"
)

func filterUpdateList(list []db.Upgrade, filter upgrade.Filter) []db.Upgrade {
	tmp := list[:0]

	for _, pkg := range list {
		if filter(pkg) {
			tmp = append(tmp, pkg)
		}
	}

	return tmp
}

// upList returns lists of packages to upgrade from each source.
func upList(ctx context.Context, warnings *query.AURWarnings, dbExecutor db.Executor, enableDowngrade bool,
	filter upgrade.Filter) (aurUp, repoUp upgrade.UpSlice, err error) {
	remote, remoteNames := query.GetRemotePackages(dbExecutor)

	var (
		wg        sync.WaitGroup
		develUp   upgrade.UpSlice
		repoSlice []db.Upgrade
		errs      multierror.MultiError
	)

	aurdata := make(map[string]*aur.Pkg)

	for _, pkg := range remote {
		if pkg.ShouldIgnore() {
			warnings.Ignore.Set(pkg.Name())
		}
	}

	if config.Runtime.Mode.AtLeastRepo() {
		text.OperationInfoln(gotext.Get("Searching databases for updates..."))
		wg.Add(1)

		go func() {
			repoSlice, err = dbExecutor.RepoUpgrades(enableDowngrade)
			errs.Add(err)
			wg.Done()
		}()
	}

	if config.Runtime.Mode.AtLeastAUR() {
		text.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		var _aurdata []*aur.Pkg
		_aurdata, err = query.AURInfo(ctx, config.Runtime.AURClient, remoteNames, warnings, config.RequestSplitN)
		errs.Add(err)

		if err == nil {
			for _, pkg := range _aurdata {
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)

			go func() {
				aurUp = upgrade.UpAUR(remote, aurdata, config.TimeUpdate)

				wg.Done()
			}()

			if config.Devel {
				text.OperationInfoln(gotext.Get("Checking development packages..."))
				wg.Add(1)

				go func() {
					develUp = upgrade.UpDevel(ctx, remote, aurdata, config.Runtime.VCSStore)

					wg.Done()
				}()
			}
		}
	}

	wg.Wait()

	printLocalNewerThanAUR(remote, aurdata)

	names := make(stringset.StringSet)

	for _, up := range develUp.Up {
		names.Set(up.Name)
	}

	for _, up := range aurUp.Up {
		if !names.Get(up.Name) {
			develUp.Up = append(develUp.Up, up)
		}
	}

	aurUp = develUp
	aurUp.Repos = []string{"aur", "devel"}

	repoUp = upgrade.UpSlice{Up: repoSlice, Repos: dbExecutor.Repos()}

	aurUp.Up = filterUpdateList(aurUp.Up, filter)
	repoUp.Up = filterUpdateList(repoUp.Up, filter)

	return aurUp, repoUp, errs.Return()
}

func printLocalNewerThanAUR(
	remote []alpm.IPackage, aurdata map[string]*aur.Pkg) {
	for _, pkg := range remote {
		aurPkg, ok := aurdata[pkg.Name()]
		if !ok {
			continue
		}

		left, right := upgrade.GetVersionDiff(pkg.Version(), aurPkg.Version)

		if !isDevelPackage(pkg) && alpm.VerCmp(pkg.Version(), aurPkg.Version) > 0 {
			text.Warnln(gotext.Get("%s: local (%s) is newer than AUR (%s)",
				text.Cyan(pkg.Name()),
				left, right,
			))
		}
	}
}

func isDevelName(name string) bool {
	for _, suffix := range []string{"git", "svn", "hg", "bzr", "nightly", "insiders-bin"} {
		if strings.HasSuffix(name, "-"+suffix) {
			return true
		}
	}

	return strings.Contains(name, "-always-")
}

func isDevelPackage(pkg alpm.IPackage) bool {
	return isDevelName(pkg.Name()) || isDevelName(pkg.Base())
}

// upgradePkgsMenu handles updating the cache and installing updates.
func upgradePkgsMenu(aurUp, repoUp upgrade.UpSlice) (stringset.StringSet, []string, error) {
	ignore := make(stringset.StringSet)
	targets := []string{}

	allUpLen := len(repoUp.Up) + len(aurUp.Up)
	if allUpLen == 0 {
		return ignore, nil, nil
	}

	if !config.UpgradeMenu {
		for _, pkg := range aurUp.Up {
			targets = append(targets, pkg.Name)
		}

		return ignore, targets, nil
	}

	sort.Sort(repoUp)
	sort.Sort(aurUp)

	allUp := upgrade.UpSlice{Up: append(repoUp.Up, aurUp.Up...), Repos: append(repoUp.Repos, aurUp.Repos...)}

	fmt.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold(gotext.Get("Packages to upgrade.")))
	allUp.Print()

	text.Infoln(gotext.Get("Packages to exclude: (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name)"))

	numbers, err := getInput(config.AnswerUpgrade)
	if err != nil {
		return nil, nil, err
	}

	// upgrade menu asks you which packages to NOT upgrade so in this case
	// include and exclude are kind of swapped
	include, exclude, otherInclude, otherExclude := intrange.ParseNumberMenu(numbers)

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoUp.Up {
		if isInclude && otherInclude.Get(pkg.Repository) {
			ignore.Set(pkg.Name)
		}

		if isInclude && !include.Get(len(repoUp.Up)-i+len(aurUp.Up)) {
			targets = append(targets, pkg.Name)
			continue
		}

		if !isInclude && (exclude.Get(len(repoUp.Up)-i+len(aurUp.Up)) || otherExclude.Get(pkg.Repository)) {
			targets = append(targets, pkg.Name)
			continue
		}

		ignore.Set(pkg.Name)
	}

	for i, pkg := range aurUp.Up {
		if isInclude && otherInclude.Get(pkg.Repository) {
			continue
		}

		if isInclude && !include.Get(len(aurUp.Up)-i) {
			targets = append(targets, "aur/"+pkg.Name)
		}

		if !isInclude && (exclude.Get(len(aurUp.Up)-i) || otherExclude.Get(pkg.Repository)) {
			targets = append(targets, "aur/"+pkg.Name)
		}
	}

	return ignore, targets, err
}

// Targets for sys upgrade.
func sysupgradeTargets(ctx context.Context, dbExecutor db.Executor,
	enableDowngrade bool) (stringset.StringSet, []string, error) {
	warnings := query.NewWarnings()

	aurUp, repoUp, err := upList(ctx, warnings, dbExecutor, enableDowngrade,
		func(upgrade.Upgrade) bool { return true })
	if err != nil {
		return nil, nil, err
	}

	warnings.Print()

	ignore, targets, errUp := upgradePkgsMenu(aurUp, repoUp)

	return ignore, targets, errUp
}
