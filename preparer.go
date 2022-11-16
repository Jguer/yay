package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/text"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type Preparer struct {
	dbExecutor db.Executor
	cmdBuilder exe.ICmdBuilder
	config     *settings.Configuration

	makeDeps []string
}

func (preper *Preparer) ShouldCleanAURDirs(pkgBuildDirs map[string]string) PostInstallHookFunc {
	if !preper.config.CleanAfter {
		return nil
	}

	text.Debugln("added post install hook to clean up AUR dirs", pkgBuildDirs)

	return func(ctx context.Context) error {
		cleanAfter(ctx, preper.config.Runtime.CmdBuilder, pkgBuildDirs)
		return nil
	}
}

func (preper *Preparer) ShouldCleanMakeDeps() PostInstallHookFunc {
	if len(preper.makeDeps) == 0 {
		return nil
	}

	switch preper.config.RemoveMake {
	case "yes":
		break
	case "no":
		return nil
	default:
		if !text.ContinueTask(os.Stdin, gotext.Get("Remove make dependencies after install?"), false, settings.NoConfirm) {
			return nil
		}
	}

	text.Debugln("added post install hook to clean up AUR makedeps", preper.makeDeps)

	return func(ctx context.Context) error {
		return removeMake(ctx, preper.config.Runtime.CmdBuilder, preper.makeDeps)
	}
}

func (preper *Preparer) Present(w io.Writer, targets []map[string]*dep.InstallInfo) error {
	pkgsBySourceAndReason := map[string]map[string][]string{}

	for _, layer := range targets {
		for pkgName, info := range layer {
			source := dep.SourceNames[info.Source]
			reason := dep.ReasonNames[info.Reason]

			var pkgStr string
			if info.Version != "" {
				pkgStr = text.Cyan(fmt.Sprintf("%s-%s", pkgName, info.Version))
			} else {
				pkgStr = text.Cyan(pkgName)
			}

			if _, ok := pkgsBySourceAndReason[source]; !ok {
				pkgsBySourceAndReason[source] = map[string][]string{}
			}

			pkgsBySourceAndReason[source][reason] = append(pkgsBySourceAndReason[source][reason], pkgStr)

			if info.Reason == dep.MakeDep {
				preper.makeDeps = append(preper.makeDeps, pkgName)
			}
		}
	}

	for source, pkgsByReason := range pkgsBySourceAndReason {
		for reason, pkgs := range pkgsByReason {
			fmt.Fprintf(w, text.Bold("%s %s (%d):")+" %s\n",
				source,
				reason,
				len(pkgs),
				strings.Join(pkgs, ", "))
		}
	}

	return nil
}

func (preper *Preparer) PrepareWorkspace(ctx context.Context, targets []map[string]*dep.InstallInfo) (map[string]string, error) {
	aurBases := mapset.NewThreadUnsafeSet[string]()
	pkgBuildDirs := make(map[string]string, len(targets))

	for _, layer := range targets {
		for pkgName, info := range layer {
			if info.Source == dep.AUR {
				pkgBase := *info.AURBase
				aurBases.Add(pkgBase)
				pkgBuildDirs[pkgName] = filepath.Join(config.BuildDir, pkgBase)
			} else if info.Source == dep.SrcInfo {
				pkgBuildDirs[pkgName] = *info.SrcinfoPath
			}
		}
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		preper.cmdBuilder, aurBases.ToSlice(), config.AURURL, config.BuildDir, false); errA != nil {
		return nil, errA
	}

	if errP := downloadPKGBUILDSourceFanout(ctx, config.Runtime.CmdBuilder,
		pkgBuildDirs, false, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}

	return pkgBuildDirs, nil
}

func (preper *Preparer) MenuHandler(targets []map[string]*dep.InstallInfo, pkgBuildDirs map[string]string) error {
	// if errCleanMenu := menus.Clean(config.CleanMenu,
	// 	config.BuildDir, do.Aur,
	// 	remoteNamesCache, settings.NoConfirm, config.AnswerClean); errCleanMenu != nil {
	// 	if errors.As(errCleanMenu, &settings.ErrUserAbort{}) {
	// 		return errCleanMenu
	// 	}

	// 	text.Errorln(errCleanMenu)
	// }

	return nil
}
