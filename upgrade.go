package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/intrange"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/stringset"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/leonelquinteros/gotext"
)

func filterUpdateList(list []db.Upgrade, filter upgrade.Filter) []db.Upgrade {
	tmp := list[:0]

	for i := range list {
		up := &list[i]
		if filter(up) {
			tmp = append(tmp, *up)
		}
	}

	return tmp
}

// upList returns lists of packages to upgrade from each source.
func upList(ctx context.Context, cfg *settings.Configuration,
	warnings *query.AURWarnings, dbExecutor db.Executor, enableDowngrade bool,
	filter upgrade.Filter,
) (aurUp, repoUp upgrade.UpSlice, err error) {
	remote := dbExecutor.InstalledRemotePackages()
	remoteNames := dbExecutor.InstalledRemotePackageNames()

	var (
		wg           sync.WaitGroup
		develUp      upgrade.UpSlice
		syncUpgrades map[string]db.SyncUpgrade
		errs         multierror.MultiError
	)

	aurdata := make(map[string]*aur.Pkg)

	for _, pkg := range remote {
		if pkg.ShouldIgnore() {
			warnings.Ignore.Set(pkg.Name())
		}
	}

	if cfg.Mode.AtLeastRepo() {
		text.OperationInfoln(gotext.Get("Searching databases for updates..."))
		wg.Add(1)

		go func() {
			syncUpgrades, err = dbExecutor.SyncUpgrades(enableDowngrade)
			errs.Add(err)
			wg.Done()
		}()
	}

	if cfg.Mode.AtLeastAUR() {
		text.OperationInfoln(gotext.Get("Searching AUR for updates..."))

		var _aurdata []aur.Pkg
		_aurdata, err = query.AURInfo(ctx, cfg.Runtime.AURClient, remoteNames, warnings, cfg.RequestSplitN)

		errs.Add(err)

		if err == nil {
			for i := range _aurdata {
				pkg := &_aurdata[i]
				aurdata[pkg.Name] = pkg
			}

			wg.Add(1)

			go func() {
				aurUp = upgrade.UpAUR(cfg.Runtime.Logger, remote, aurdata, cfg.TimeUpdate, enableDowngrade)

				wg.Done()
			}()

			if cfg.Devel {
				text.OperationInfoln(gotext.Get("Checking development packages..."))
				wg.Add(1)

				go func() {
					develUp = upgrade.UpDevel(ctx, cfg.Runtime.Logger, remote, aurdata, cfg.Runtime.VCSStore)

					cfg.Runtime.VCSStore.CleanOrphans(remote)
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

	repoUp = upgrade.UpSlice{
		Up:    make([]db.Upgrade, 0, len(syncUpgrades)),
		Repos: dbExecutor.Repos(),
	}
	for _, up := range syncUpgrades {
		dbUp := db.Upgrade{
			Name:          up.Package.Name(),
			RemoteVersion: up.Package.Version(),
			Repository:    up.Package.DB().Name(),
			Base:          up.Package.Base(),
			LocalVersion:  up.LocalVersion,
			Reason:        up.Reason,
		}
		if filter != nil && !filter(&dbUp) {
			continue
		}

		repoUp.Up = append(repoUp.Up, dbUp)
	}

	aurUp.Up = filterUpdateList(aurUp.Up, filter)
	repoUp.Up = filterUpdateList(repoUp.Up, filter)

	return aurUp, repoUp, errs.Return()
}

func printLocalNewerThanAUR(
	remote map[string]alpm.IPackage, aurdata map[string]*aur.Pkg,
) {
	for name, pkg := range remote {
		aurPkg, ok := aurdata[name]
		if !ok {
			continue
		}

		left, right := upgrade.GetVersionDiff(pkg.Version(), aurPkg.Version)

		if !isDevelPackage(pkg) && db.VerCmp(pkg.Version(), aurPkg.Version) > 0 {
			text.Warnln(gotext.Get("%s: local (%s) is newer than AUR (%s)",
				text.Cyan(name),
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
func upgradePkgsMenu(cfg *settings.Configuration, aurUp, repoUp upgrade.UpSlice) (stringset.StringSet, []string, error) {
	ignore := make(stringset.StringSet)
	targets := []string{}

	allUpLen := len(repoUp.Up) + len(aurUp.Up)
	if allUpLen == 0 {
		return ignore, nil, nil
	}

	if !cfg.UpgradeMenu {
		for _, pkg := range aurUp.Up {
			targets = append(targets, pkg.Repository+"/"+pkg.Name)
		}

		return ignore, targets, nil
	}

	sort.Sort(repoUp)
	sort.Sort(aurUp)

	allUp := upgrade.UpSlice{Up: append(repoUp.Up, aurUp.Up...), Repos: append(repoUp.Repos, aurUp.Repos...)}

	fmt.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold(gotext.Get("Packages to upgrade.")))
	allUp.Print(cfg.Runtime.Logger)

	text.Infoln(gotext.Get("Packages to exclude") + " (eg: \"1 2 3\", \"1-3\", \"^4\" or repo name):")

	numbers, err := text.GetInput(os.Stdin, cfg.AnswerUpgrade, settings.NoConfirm)
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
			targets = append(targets, pkg.Repository+"/"+pkg.Name)
			continue
		}

		if !isInclude && (exclude.Get(len(repoUp.Up)-i+len(aurUp.Up)) || otherExclude.Get(pkg.Repository)) {
			targets = append(targets, pkg.Repository+"/"+pkg.Name)
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
func sysupgradeTargets(ctx context.Context, cfg *settings.Configuration, dbExecutor db.Executor,
	enableDowngrade bool,
) (stringset.StringSet, []string, error) {
	warnings := query.NewWarnings()

	aurUp, repoUp, err := upList(ctx, cfg, warnings, dbExecutor, enableDowngrade,
		func(*upgrade.Upgrade) bool { return true })
	if err != nil {
		return nil, nil, err
	}

	warnings.Print()

	return upgradePkgsMenu(cfg, aurUp, repoUp)
}
