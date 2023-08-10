package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	aur "github.com/Jguer/aur"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
	"golang.org/x/sys/unix"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/upgrade"
)

// printInfo prints package info like pacman -Si.
func printInfo(logger *text.Logger, config *settings.Configuration, a *aur.Pkg, extendedInfo bool) {
	printInfoValue(logger, gotext.Get("Repository"), "aur")
	printInfoValue(logger, gotext.Get("Name"), a.Name)
	printInfoValue(logger, gotext.Get("Version"), a.Version)
	printInfoValue(logger, gotext.Get("Description"), a.Description)
	printInfoValue(logger, gotext.Get("URL"), a.URL)
	printInfoValue(logger, gotext.Get("Licenses"), a.License...)
	printInfoValue(logger, gotext.Get("Groups"), a.Groups...)
	printInfoValue(logger, gotext.Get("Provides"), a.Provides...)
	printInfoValue(logger, gotext.Get("Depends On"), a.Depends...)
	printInfoValue(logger, gotext.Get("Optional Deps"), a.OptDepends...)
	printInfoValue(logger, gotext.Get("Make Deps"), a.MakeDepends...)
	printInfoValue(logger, gotext.Get("Check Deps"), a.CheckDepends...)
	printInfoValue(logger, gotext.Get("Conflicts With"), a.Conflicts...)
	printInfoValue(logger, gotext.Get("Replaces"), a.Replaces...)
	printInfoValue(logger, gotext.Get("AUR URL"), config.AURURL+"/packages/"+a.Name)
	printInfoValue(logger, gotext.Get("First Submitted"), text.FormatTimeQuery(a.FirstSubmitted))
	printInfoValue(logger, gotext.Get("Keywords"), a.Keywords...)
	printInfoValue(logger, gotext.Get("Last Modified"), text.FormatTimeQuery(a.LastModified))
	printInfoValue(logger, gotext.Get("Maintainer"), a.Maintainer)
	printInfoValue(logger, gotext.Get("Popularity"), fmt.Sprintf("%f", a.Popularity))
	printInfoValue(logger, gotext.Get("Votes"), fmt.Sprintf("%d", a.NumVotes))

	if a.OutOfDate != 0 {
		printInfoValue(logger, gotext.Get("Out-of-date"), text.FormatTimeQuery(a.OutOfDate))
	} else {
		printInfoValue(logger, gotext.Get("Out-of-date"), "No")
	}

	if extendedInfo {
		printInfoValue(logger, "ID", fmt.Sprintf("%d", a.ID))
		printInfoValue(logger, gotext.Get("Package Base ID"), fmt.Sprintf("%d", a.PackageBaseID))
		printInfoValue(logger, gotext.Get("Package Base"), a.PackageBase)
		printInfoValue(logger, gotext.Get("Snapshot URL"), config.AURURL+a.URLPath)
	}

	logger.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages(logger *text.Logger, dbExecutor db.Executor) {
	pkgS := dbExecutor.BiggestPackages()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		logger.Printf("%s: %s\n", text.Bold(pkgS[i].Name()), text.Cyan(text.Human(pkgS[i].ISize())))
	}
}

// localStatistics prints installed packages statistics.
func localStatistics(ctx context.Context, run *runtime.Runtime, dbExecutor db.Executor) error {
	info := statistics(run, dbExecutor)

	remoteNames := dbExecutor.InstalledRemotePackageNames()
	remote := dbExecutor.InstalledRemotePackages()
	run.Logger.Infoln(gotext.Get("Yay version v%s", yayVersion))
	run.Logger.Println(text.Bold(text.Cyan("===========================================")))
	run.Logger.Infoln(gotext.Get("Total installed packages: %s", text.Cyan(strconv.Itoa(info.Totaln))))
	run.Logger.Infoln(gotext.Get("Foreign installed packages: %s", text.Cyan(strconv.Itoa(len(remoteNames)))))
	run.Logger.Infoln(gotext.Get("Explicitly installed packages: %s", text.Cyan(strconv.Itoa(info.Expln))))
	run.Logger.Infoln(gotext.Get("Total Size occupied by packages: %s", text.Cyan(text.Human(info.TotalSize))))

	for path, size := range info.pacmanCaches {
		run.Logger.Infoln(gotext.Get("Size of pacman cache %s: %s", path, text.Cyan(text.Human(size))))
	}

	run.Logger.Infoln(gotext.Get("Size of yay cache %s: %s", run.Cfg.BuildDir, text.Cyan(text.Human(info.yayCache))))
	run.Logger.Println(text.Bold(text.Cyan("===========================================")))
	run.Logger.Infoln(gotext.Get("Ten biggest packages:"))
	biggestPackages(run.Logger, dbExecutor)
	run.Logger.Println(text.Bold(text.Cyan("===========================================")))

	aurData, err := run.AURClient.Get(ctx, &aur.Query{
		Needles: remoteNames,
		By:      aur.Name,
	})
	if err != nil {
		return err
	}

	warnings := query.NewWarnings(run.Logger.Child("warnings"))
	for i := range aurData {
		warnings.AddToWarnings(remote, &aurData[i])
	}

	warnings.Print()

	return nil
}

func printUpdateList(ctx context.Context, run *runtime.Runtime, cmdArgs *parser.Arguments,
	dbExecutor db.Executor, enableDowngrade bool, filter upgrade.Filter,
) error {
	quietMode := cmdArgs.ExistsArg("q", "quiet")

	// TODO: handle quiet mode in a better way
	logger := text.NewLogger(io.Discard, os.Stderr, os.Stdin, run.Cfg.Debug, "update-list")
	dbExecutor.SetLogger(logger.Child("db"))
	oldNoConfirm := settings.NoConfirm
	settings.NoConfirm = true
	// restoring global NoConfirm to make tests work properly
	defer func() { settings.NoConfirm = oldNoConfirm }()

	targets := mapset.NewThreadUnsafeSet(cmdArgs.Targets...)
	grapher := dep.NewGrapher(dbExecutor, run.AURClient, run.CmdBuilder, false, true,
		false, false, cmdArgs.ExistsArg("needed"), logger.Child("grapher"))

	upService := upgrade.NewUpgradeService(
		grapher, run.AURClient, dbExecutor, run.VCSStore,
		run.Cfg, true, logger.Child("upgrade"))

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
				run.Logger.Printf("%s\n", pkgName)
			} else {
				run.Logger.Printf("%s %s -> %s\n", text.Bold(pkgName), text.Bold(text.Green(ii.LocalVersion)),
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
			run.Logger.Errorln(gotext.Get("package '%s' was not found", pkgName))
			missing = true
		}
		return false
	})

	if missing || noUpdates {
		return fmt.Errorf("")
	}

	return nil
}

func printInfoValue(logger *text.Logger, key string, values ...string) {
	const (
		keyLength  = 32
		delimCount = 2
	)

	specialWordsCount := 0

	for _, runeValue := range key {
		// CJK handling: the character 'ー' is Katakana
		// but if use unicode.Katakana, it will return false
		if unicode.IsOneOf([]*unicode.RangeTable{
			unicode.Han,
			unicode.Hiragana,
			unicode.Katakana,
			unicode.Hangul,
		}, runeValue) || runeValue == 'ー' {
			specialWordsCount++
		}
	}

	keyTextCount := specialWordsCount - keyLength + delimCount
	str := fmt.Sprintf(text.Bold("%-*s: "), keyTextCount, key)

	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		logger.Printf("%s%s\n", str, gotext.Get("None"))
		return
	}

	maxCols := getColumnCount()
	cols := keyLength + len(values[0])
	str += values[0]

	for _, value := range values[1:] {
		if maxCols > keyLength && cols+len(value)+delimCount >= maxCols {
			cols = keyLength
			str += "\n" + strings.Repeat(" ", keyLength)
		} else if cols != keyLength {
			str += strings.Repeat(" ", delimCount)
			cols += delimCount
		}

		str += value
		cols += len(value)
	}

	logger.Println(str)
}

var cachedColumnCount = -1

func getColumnCount() int {
	if cachedColumnCount > 0 {
		return cachedColumnCount
	}

	if count, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil {
		cachedColumnCount = count
		return cachedColumnCount
	}

	if ws, err := unix.IoctlGetWinsize(syscall.Stdout, unix.TIOCGWINSZ); err == nil {
		cachedColumnCount = int(ws.Col)
		return cachedColumnCount
	}

	return 80
}
