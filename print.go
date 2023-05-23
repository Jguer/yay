package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	aur "github.com/Jguer/aur"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"
)

// PrintInfo prints package info like pacman -Si.
func PrintInfo(config *settings.Configuration, a *aur.Pkg, extendedInfo bool) {
	text.PrintInfoValue(gotext.Get("Repository"), "aur")
	text.PrintInfoValue(gotext.Get("Name"), a.Name)
	text.PrintInfoValue(gotext.Get("Keywords"), a.Keywords...)
	text.PrintInfoValue(gotext.Get("Version"), a.Version)
	text.PrintInfoValue(gotext.Get("Description"), a.Description)
	text.PrintInfoValue(gotext.Get("URL"), a.URL)
	text.PrintInfoValue(gotext.Get("AUR URL"), config.AURURL+"/packages/"+a.Name)
	text.PrintInfoValue(gotext.Get("Groups"), a.Groups...)
	text.PrintInfoValue(gotext.Get("Licenses"), a.License...)
	text.PrintInfoValue(gotext.Get("Provides"), a.Provides...)
	text.PrintInfoValue(gotext.Get("Depends On"), a.Depends...)
	text.PrintInfoValue(gotext.Get("Make Deps"), a.MakeDepends...)
	text.PrintInfoValue(gotext.Get("Check Deps"), a.CheckDepends...)
	text.PrintInfoValue(gotext.Get("Optional Deps"), a.OptDepends...)
	text.PrintInfoValue(gotext.Get("Conflicts With"), a.Conflicts...)
	text.PrintInfoValue(gotext.Get("Maintainer"), a.Maintainer)
	text.PrintInfoValue(gotext.Get("Votes"), fmt.Sprintf("%d", a.NumVotes))
	text.PrintInfoValue(gotext.Get("Popularity"), fmt.Sprintf("%f", a.Popularity))
	text.PrintInfoValue(gotext.Get("First Submitted"), text.FormatTimeQuery(a.FirstSubmitted))
	text.PrintInfoValue(gotext.Get("Last Modified"), text.FormatTimeQuery(a.LastModified))

	if a.OutOfDate != 0 {
		text.PrintInfoValue(gotext.Get("Out-of-date"), text.FormatTimeQuery(a.OutOfDate))
	} else {
		text.PrintInfoValue(gotext.Get("Out-of-date"), "No")
	}

	if extendedInfo {
		text.PrintInfoValue("ID", fmt.Sprintf("%d", a.ID))
		text.PrintInfoValue(gotext.Get("Package Base ID"), fmt.Sprintf("%d", a.PackageBaseID))
		text.PrintInfoValue(gotext.Get("Package Base"), a.PackageBase)
		text.PrintInfoValue(gotext.Get("Snapshot URL"), config.AURURL+a.URLPath)
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages(dbExecutor db.Executor) {
	pkgS := dbExecutor.BiggestPackages()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Printf("%s: %s\n", text.Bold(pkgS[i].Name()), text.Cyan(text.Human(pkgS[i].ISize())))
	}
}

// localStatistics prints installed packages statistics.
func localStatistics(ctx context.Context, cfg *settings.Configuration, dbExecutor db.Executor) error {
	info := statistics(cfg, dbExecutor)

	remoteNames := dbExecutor.InstalledRemotePackageNames()
	remote := dbExecutor.InstalledRemotePackages()
	text.Infoln(gotext.Get("Yay version v%s", yayVersion))
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	text.Infoln(gotext.Get("Total installed packages: %s", text.Cyan(strconv.Itoa(info.Totaln))))
	text.Infoln(gotext.Get("Foreign installed packages: %s", text.Cyan(strconv.Itoa(len(remoteNames)))))
	text.Infoln(gotext.Get("Explicitly installed packages: %s", text.Cyan(strconv.Itoa(info.Expln))))
	text.Infoln(gotext.Get("Total Size occupied by packages: %s", text.Cyan(text.Human(info.TotalSize))))

	for path, size := range info.pacmanCaches {
		text.Infoln(gotext.Get("Size of pacman cache %s: %s", path, text.Cyan(text.Human(size))))
	}

	text.Infoln(gotext.Get("Size of yay cache %s: %s", cfg.BuildDir, text.Cyan(text.Human(info.yayCache))))
	fmt.Println(text.Bold(text.Cyan("===========================================")))
	text.Infoln(gotext.Get("Ten biggest packages:"))
	biggestPackages(dbExecutor)
	fmt.Println(text.Bold(text.Cyan("===========================================")))

	aurData, err := cfg.Runtime.AURClient.Get(ctx, &aur.Query{
		Needles: remoteNames,
		By:      aur.Name,
	})
	if err != nil {
		return err
	}

	warnings := query.NewWarnings(cfg.Runtime.Logger.Child("print"))
	for i := range aurData {
		warnings.AddToWarnings(remote, &aurData[i])
	}

	warnings.Print()

	return nil
}

func printUpdateList(ctx context.Context, cfg *settings.Configuration, cmdArgs *parser.Arguments,
	dbExecutor db.Executor, enableDowngrade bool, filter upgrade.Filter,
) error {
	quietMode := cmdArgs.ExistsArg("q", "quiet")

	// TODO: handle quiet mode in a better way
	logger := text.NewLogger(io.Discard, os.Stderr, os.Stdin, cfg.Debug, "update-list")
	dbExecutor.SetLogger(logger.Child("db"))
	oldNoConfirm := settings.NoConfirm
	settings.NoConfirm = true
	// restoring global NoConfirm to make tests work properly
	defer func() { settings.NoConfirm = oldNoConfirm }()

	targets := mapset.NewThreadUnsafeSet(cmdArgs.Targets...)
	grapher := dep.NewGrapher(dbExecutor, cfg.Runtime.AURClient, false, true,
		false, false, cmdArgs.ExistsArg("needed"), logger.Child("grapher"))

	upService := upgrade.NewUpgradeService(
		grapher, cfg.Runtime.AURClient, dbExecutor, cfg.Runtime.VCSStore,
		cfg, true, logger.Child("upgrade"))

	graph, errSysUp := upService.GraphUpgrades(ctx, nil,
		enableDowngrade, filter)
	if errSysUp != nil {
		return errSysUp
	}

	if graph.Len() == 0 {
		return fmt.Errorf("")
	}

	noTargets := targets.Cardinality() == 0
	foreignFilter := cmdArgs.ExistsArg("m", "foreign")
	nativeFilter := cmdArgs.ExistsArg("n", "native")

	noUpdates := true
	_ = graph.ForEach(func(pkgName string, ii *dep.InstallInfo) error {
		if !ii.Upgrade {
			return nil
		}

		if noTargets || targets.Contains(pkgName) {
			if ii.Source == dep.Sync && foreignFilter {
				return nil
			} else if ii.Source == dep.AUR && nativeFilter {
				return nil
			}

			if quietMode {
				fmt.Printf("%s\n", pkgName)
			} else {
				fmt.Printf("%s %s -> %s\n", text.Bold(pkgName), text.Bold(text.Green(ii.LocalVersion)),
					text.Bold(text.Green(ii.Version)))
			}

			targets.Remove(pkgName)
			noUpdates = false
		}

		return nil
	})

	missing := false
	targets.Each(func(pkgName string) bool {
		if dbExecutor.LocalPackage(pkgName) == nil {
			cfg.Runtime.Logger.Errorln(gotext.Get("package '%s' was not found", pkgName))
			missing = true
		}
		return false
	})

	if missing || noUpdates {
		return fmt.Errorf("")
	}

	return nil
}
