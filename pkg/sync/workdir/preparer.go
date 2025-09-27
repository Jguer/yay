package workdir

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
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/sync/build"
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

type HookFn func(ctx context.Context, run *runtime.Runtime, w io.Writer,
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
	log             *text.Logger

	makeDeps []string
}

func NewPreparerWithoutHooks(dbExecutor db.Executor, cmdBuilder exe.ICmdBuilder,
	cfg *settings.Configuration, logger *text.Logger, downloadSources bool,
) *Preparer {
	return &Preparer{
		dbExecutor:      dbExecutor,
		cmdBuilder:      cmdBuilder,
		cfg:             cfg,
		hooks:           []Hook{},
		downloadSources: downloadSources,
		log:             logger,
	}
}

func NewPreparer(dbExecutor db.Executor, cmdBuilder exe.ICmdBuilder,
	cfg *settings.Configuration, logger *text.Logger,
) *Preparer {
	preper := NewPreparerWithoutHooks(dbExecutor, cmdBuilder, cfg, logger, true)

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

func (preper *Preparer) ShouldCleanAURDirs(run *runtime.Runtime, pkgBuildDirs map[string]string) build.PostInstallHookFunc {
	if !preper.cfg.CleanAfter || len(pkgBuildDirs) == 0 {
		return nil
	}

	preper.log.Debugln("added post install hook to clean up AUR dirs", pkgBuildDirs)

	return func(ctx context.Context) error {
		cleanAfter(ctx, run, run.CmdBuilder, pkgBuildDirs)
		return nil
	}
}

func (preper *Preparer) ShouldCleanMakeDeps(run *runtime.Runtime, cmdArgs *parser.Arguments) build.PostInstallHookFunc {
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
		if !preper.log.ContinueTask(gotext.Get("Remove make dependencies after install?"),
			isYesDefault, settings.NoConfirm) {
			return nil
		}
	}

	preper.log.Debugln("added post install hook to clean up AUR makedeps", preper.makeDeps)

	return func(ctx context.Context) error {
		return removeMake(ctx, preper.cfg, run.CmdBuilder, preper.makeDeps, cmdArgs)
	}
}

func (preper *Preparer) Run(ctx context.Context, run *runtime.Runtime,
	targets []map[string]*dep.InstallInfo,
) (pkgbuildDirsByBase map[string]string, err error) {
	preper.Present(targets)

	pkgBuildDirs, err := preper.PrepareWorkspace(ctx, run, targets)
	if err != nil {
		return nil, err
	}

	return pkgBuildDirs, nil
}

func (preper *Preparer) Present(targets []map[string]*dep.InstallInfo) {
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
			preper.log.Printf(text.Bold("%s %s (%d):")+" %s\n",
				source,
				reason,
				len(pkgs),
				strings.Join(pkgs, ", "))
		}
	}
}

func (preper *Preparer) PrepareWorkspace(ctx context.Context,
	run *runtime.Runtime, targets []map[string]*dep.InstallInfo,
) (map[string]string, error) {
	aurBasesToClone := mapset.NewThreadUnsafeSet[string]()
	pkgBuildDirsByBase := make(map[string]string, len(targets))

	for _, layer := range targets {
		for name, info := range layer {
			switch info.Source {
			case dep.AUR:
				pkgBase := *info.AURBase
				pkgBuildDir := filepath.Join(preper.cfg.BuildDir, pkgBase)
				if preper.needToCloneAURBase(info, pkgBuildDir) {
					aurBasesToClone.Add(pkgBase)
				}
				pkgBuildDirsByBase[pkgBase] = pkgBuildDir
			case dep.SrcInfo:
				pkgBase := *info.AURBase
				pkgBuildDirsByBase[pkgBase] = *info.SrcinfoPath
			case dep.CustomRepo:
				// Custom repository packages need to be copied to build directory
				pkgBase := name
				if info.CustomRepoPath != nil {
					// Copy PKGBUILD from custom repository to build directory
					buildDir := filepath.Join(preper.cfg.BuildDir, pkgBase)
					if err := preper.copyCustomRepoPkg(ctx, *info.CustomRepoPath, buildDir); err != nil {
						preper.log.Warnln("Failed to copy custom repo package:", err)
						continue
					}
					pkgBuildDirsByBase[pkgBase] = buildDir
				}
			}
		}
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		preper.cmdBuilder, preper.log.Child("download"), aurBasesToClone.ToSlice(),
		preper.cfg.AURURL, preper.cfg.BuildDir, false); errA != nil {
		return nil, errA
	}

	if !preper.downloadSources {
		return pkgBuildDirsByBase, nil
	}

	// Filter out custom repository packages from mergePkgbuilds
	aurPkgBuildDirs := make(map[string]string)
	for base, dir := range pkgBuildDirsByBase {
		// Check if this is a custom repository package by looking for .git directory
		// Custom repository packages have .git directory but no remote
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			// Check if this is a custom repository package (no remote)
			_, stderr, err := preper.cmdBuilder.Capture(
				preper.cmdBuilder.BuildGitCmd(ctx, dir, "remote", "get-url", "origin"))
			if err != nil && strings.Contains(stderr, "No such remote") {
				// This is a custom repository package, skip merge
				continue
			}
		}
		aurPkgBuildDirs[base] = dir
	}
	
	if err := mergePkgbuilds(ctx, preper.cmdBuilder, aurPkgBuildDirs); err != nil {
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
		preper.log.Errorln(errP)
	}

	return pkgBuildDirsByBase, nil
}

// copyCustomRepoPkg copies PKGBUILD and related files from custom repository to build directory
func (preper *Preparer) copyCustomRepoPkg(ctx context.Context, srcDir, dstDir string) error {
	// Create destination directory
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}
	
	// Initialize git repository in build directory
	if err := preper.cmdBuilder.Show(preper.cmdBuilder.BuildGitCmd(ctx, dstDir, "init")); err != nil {
		preper.log.Warnln("Failed to initialize git repository:", err)
	}
	
	// Copy PKGBUILD
	srcPKGBUILD := filepath.Join(srcDir, "PKGBUILD")
	dstPKGBUILD := filepath.Join(dstDir, "PKGBUILD")
	if err := preper.copyFile(srcPKGBUILD, dstPKGBUILD); err != nil {
		return fmt.Errorf("failed to copy PKGBUILD: %w", err)
	}
	
	// Copy .SRCINFO if it exists
	srcSRCINFO := filepath.Join(srcDir, ".SRCINFO")
	dstSRCINFO := filepath.Join(dstDir, ".SRCINFO")
	if _, err := os.Stat(srcSRCINFO); err == nil {
		if err := preper.copyFile(srcSRCINFO, dstSRCINFO); err != nil {
			preper.log.Warnln("Failed to copy .SRCINFO:", err)
		}
	}
	
	// Copy other files that might be needed (patches, install scripts, etc.)
	files, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		fileName := file.Name()
		// Skip PKGBUILD and .SRCINFO as they're already copied
		if fileName == "PKGBUILD" || fileName == ".SRCINFO" {
			continue
		}
		
		// Copy other files
		srcFile := filepath.Join(srcDir, fileName)
		dstFile := filepath.Join(dstDir, fileName)
		if err := preper.copyFile(srcFile, dstFile); err != nil {
			preper.log.Warnln("Failed to copy file", fileName+":", err)
		}
	}
	
	// Add and commit files to git repository
	if err := preper.cmdBuilder.Show(preper.cmdBuilder.BuildGitCmd(ctx, dstDir, "add", ".")); err != nil {
		preper.log.Warnln("Failed to add files to git:", err)
	}
	
	if err := preper.cmdBuilder.Show(preper.cmdBuilder.BuildGitCmd(ctx, dstDir, "commit", "-m", "Initial commit")); err != nil {
		preper.log.Warnln("Failed to commit files to git:", err)
	}
	
	return nil
}

// copyFile copies a single file
func (preper *Preparer) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	
	_, err = dstFile.ReadFrom(srcFile)
	return err
}

func (preper *Preparer) needToCloneAURBase(installInfo *dep.InstallInfo, pkgbuildDir string) bool {
	if preper.cfg.ReDownload == "all" {
		return true
	}

	srcinfoFile := filepath.Join(pkgbuildDir, ".SRCINFO")
	if pkgbuild, err := gosrc.ParseFile(srcinfoFile); err == nil {
		if db.VerCmp(pkgbuild.Version(), installInfo.Version) >= 0 {
			preper.log.OperationInfoln(
				gotext.Get("PKGBUILD up to date, skipping download: %s",
					text.Cyan(*installInfo.AURBase)))
			return false
		}
	}

	return true
}
