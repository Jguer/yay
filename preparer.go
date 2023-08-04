package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/menus"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type HookType string

const (
	// PreDownloadSourcesHook is called before sourcing a package
	PreDownloadSourcesHook HookType = "pre-download-sources"
)

type HookFn func(ctx context.Context, run *settings.Runtime, w io.Writer,
	pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
) error

type Hook struct {
	Name   string
	Hookfn HookFn
	Type   HookType
}

type Preparer struct {
	dbExecutor      db.Executor
	cmdBuilder      exe.ICmdBuilder
	cfg             *settings.Configuration
	hooks           []Hook
	downloadSources bool

	makeDeps []string
}

func NewPreparerWithoutHooks(dbExecutor db.Executor, cmdBuilder exe.ICmdBuilder,
	cfg *settings.Configuration, downloadSources bool,
) *Preparer {
	return &Preparer{
		dbExecutor:      dbExecutor,
		cmdBuilder:      cmdBuilder,
		cfg:             cfg,
		hooks:           []Hook{},
		downloadSources: downloadSources,
	}
}

func NewPreparer(dbExecutor db.Executor, cmdBuilder exe.ICmdBuilder,
	cfg *settings.Configuration,
) *Preparer {
	preper := NewPreparerWithoutHooks(dbExecutor, cmdBuilder, cfg, true)

	if cfg.CleanMenu {
		preper.hooks = append(preper.hooks, Hook{
			Name:   "clean",
			Hookfn: menus.CleanFn,
			Type:   PreDownloadSourcesHook,
		})
	}

	if cfg.DiffMenu {
		preper.hooks = append(preper.hooks, Hook{
			Name:   "diff",
			Hookfn: menus.DiffFn,
			Type:   PreDownloadSourcesHook,
		})
	}

	if cfg.EditMenu {
		preper.hooks = append(preper.hooks, Hook{
			Name:   "edit",
			Hookfn: menus.EditFn,
			Type:   PreDownloadSourcesHook,
		})
	}

	return preper
}

func (preper *Preparer) ShouldCleanAURDirs(run *settings.Runtime, pkgBuildDirs map[string]string) PostInstallHookFunc {
	if !preper.cfg.CleanAfter || len(pkgBuildDirs) == 0 {
		return nil
	}

	text.Debugln("added post install hook to clean up AUR dirs", pkgBuildDirs)

	return func(ctx context.Context) error {
		cleanAfter(ctx, run, run.CmdBuilder, pkgBuildDirs)
		return nil
	}
}

func (preper *Preparer) ShouldCleanMakeDeps(run *settings.Runtime, cmdArgs *parser.Arguments) PostInstallHookFunc {
	if len(preper.makeDeps) == 0 {
		return nil
	}

	switch preper.cfg.RemoveMake {
	case "yes":
		break
	case "no":
		return nil
	default:
		isYesDefault := preper.cfg.RemoveMake == "askyes"
		if !text.ContinueTask(os.Stdin, gotext.Get("Remove make dependencies after install?"),
			isYesDefault, settings.NoConfirm) {
			return nil
		}
	}

	text.Debugln("added post install hook to clean up AUR makedeps", preper.makeDeps)

	return func(ctx context.Context) error {
		return removeMake(ctx, preper.cfg, run.CmdBuilder, preper.makeDeps, cmdArgs)
	}
}

func (preper *Preparer) Run(ctx context.Context, run *settings.Runtime,
	w io.Writer, targets []map[string]*dep.InstallInfo,
) (pkgbuildDirsByBase map[string]string, err error) {
	preper.Present(w, targets)

	pkgBuildDirs, err := preper.PrepareWorkspace(ctx, run, targets)
	if err != nil {
		return nil, err
	}

	return pkgBuildDirs, nil
}

func (preper *Preparer) Present(w io.Writer, targets []map[string]*dep.InstallInfo) {
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
}

func (preper *Preparer) PrepareWorkspace(ctx context.Context,
	run *settings.Runtime, targets []map[string]*dep.InstallInfo,
) (map[string]string, error) {
	aurBasesToClone := mapset.NewThreadUnsafeSet[string]()
	pkgBuildDirsByBase := make(map[string]string, len(targets))

	for _, layer := range targets {
		for _, info := range layer {
			if info.Source == dep.AUR {
				pkgBase := *info.AURBase
				pkgBuildDir := filepath.Join(preper.cfg.BuildDir, pkgBase)
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
		preper.cfg.AURURL, preper.cfg.BuildDir, false); errA != nil {
		return nil, errA
	}

	if !preper.downloadSources {
		return pkgBuildDirsByBase, nil
	}

	if err := mergePkgbuilds(ctx, preper.cmdBuilder, pkgBuildDirsByBase); err != nil {
		return nil, err
	}

	remoteNames := preper.dbExecutor.InstalledRemotePackageNames()
	remoteNamesCache := mapset.NewThreadUnsafeSet(remoteNames...)
	for _, hookFn := range preper.hooks {
		if hookFn.Type == PreDownloadSourcesHook {
			if err := hookFn.Hookfn(ctx, run, os.Stdout, pkgBuildDirsByBase, remoteNamesCache); err != nil {
				return nil, err
			}
		}
	}

	if errP := downloadPKGBUILDSourceFanout(ctx, preper.cmdBuilder,
		pkgBuildDirsByBase, false, preper.cfg.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}

	return pkgBuildDirsByBase, nil
}

func (preper *Preparer) needToCloneAURBase(installInfo *dep.InstallInfo, pkgbuildDir string) bool {
	if preper.cfg.ReDownload == "all" {
		return true
	}

	srcinfoFile := filepath.Join(pkgbuildDir, ".SRCINFO")
	if pkgbuild, err := gosrc.ParseFile(srcinfoFile); err == nil {
		if db.VerCmp(pkgbuild.Version(), installInfo.Version) >= 0 {
			text.OperationInfoln(
				gotext.Get("PKGBUILD up to date, skipping download: %s",
					text.Cyan(*installInfo.AURBase)))
			return false
		}
	}

	return true
}
