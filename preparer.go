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
	"github.com/Jguer/yay/v11/pkg/menus"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/text"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type PostDownloadHookFunc func(ctx context.Context, config *settings.Configuration, w io.Writer, pkgbuildDirsByBase map[string]string) error

type Preparer struct {
	dbExecutor        db.Executor
	cmdBuilder        exe.ICmdBuilder
	config            *settings.Configuration
	postDownloadHooks []PostDownloadHookFunc

	makeDeps []string
}

func NewPreparer(dbExecutor db.Executor, cmdBuilder exe.ICmdBuilder, config *settings.Configuration) *Preparer {
	preper := &Preparer{
		dbExecutor:        dbExecutor,
		cmdBuilder:        cmdBuilder,
		config:            config,
		postDownloadHooks: []PostDownloadHookFunc{},
	}

	if config.CleanMenu {
		preper.postDownloadHooks = append(preper.postDownloadHooks, menus.CleanFn)
	}

	if config.DiffMenu {
		preper.postDownloadHooks = append(preper.postDownloadHooks, menus.DiffFn)
	}

	if config.EditMenu {
		preper.postDownloadHooks = append(preper.postDownloadHooks, menus.EditFn)
	}

	return preper
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
	aurBasesToClone := mapset.NewThreadUnsafeSet[string]()
	pkgBuildDirsByBase := make(map[string]string, len(targets))

	for _, layer := range targets {
		for _, info := range layer {
			if info.Source == dep.AUR {
				pkgBase := *info.AURBase
				pkgBuildDir := filepath.Join(preper.config.BuildDir, pkgBase)
				if preper.needToCloneAURBase(info, pkgBuildDir) {
					aurBasesToClone.Add(pkgBase)
				}
				pkgBuildDirsByBase[pkgBase] = pkgBuildDir
			} else if info.Source == dep.SrcInfo {
				pkgBase := *info.AURBase
				pkgBuildDirsByBase[pkgBase] = *info.SrcinfoPath
			}
		}
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		preper.cmdBuilder, aurBasesToClone.ToSlice(),
		config.AURURL, config.BuildDir, false); errA != nil {
		return nil, errA
	}

	if errP := downloadPKGBUILDSourceFanout(ctx, config.Runtime.CmdBuilder,
		pkgBuildDirsByBase, false, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}

	for _, hookFn := range preper.postDownloadHooks {
		if err := hookFn(ctx, preper.config, os.Stdout, pkgBuildDirsByBase); err != nil {
			return nil, err
		}
	}

	return pkgBuildDirsByBase, nil
}

func (preper *Preparer) needToCloneAURBase(installInfo *dep.InstallInfo, pkgbuildDir string) bool {
	if preper.config.ReDownload == "all" {
		return true
	}

	srcinfoFile := filepath.Join(pkgbuildDir, ".SRCINFO")
	if pkgbuild, err := gosrc.ParseFile(srcinfoFile); err == nil {
		if db.VerCmp(pkgbuild.Version(), installInfo.Version) >= 0 {
			return false
		}
	}

	return true
}
