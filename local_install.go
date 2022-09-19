// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
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
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
)

var ErrInstallRepoPkgs = errors.New(gotext.Get("error installing repo packages"))

func installLocalPKGBUILD(
	ctx context.Context,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
) error {
	aurCache, err := metadata.NewAURCache(filepath.Join(config.BuildDir, "aur.json"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve working directory"))
	}

	if len(cmdArgs.Targets) > 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	if len(cmdArgs.Targets) == 1 {
		wd = cmdArgs.Targets[0]
	}

	pkgbuild, err := gosrc.ParseFile(filepath.Join(wd, ".SRCINFO"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
	}

	grapher := dep.NewGrapher(dbExecutor, aurCache, false, settings.NoConfirm, os.Stdout)

	graph, err := grapher.GraphFromSrcInfo(wd, pkgbuild)
	if err != nil {
		return err
	}

	topoSorted := graph.TopoSortedLayerMap()
	fmt.Println(topoSorted, len(topoSorted))

	preparer := &Preparer{
		dbExecutor: dbExecutor,
		cmdBuilder: config.Runtime.CmdBuilder,
		config:     config,
	}
	installer := &Installer{dbExecutor: dbExecutor}

	err = preparer.Present(os.Stdout, topoSorted)
	if err != nil {
		return err
	}

	cleanFunc := preparer.ShouldCleanMakeDeps(ctx)
	if cleanFunc != nil {
		installer.AddPostInstallHook(cleanFunc)
	}

	pkgBuildDirs, err := preparer.PrepareWorkspace(ctx, topoSorted)
	if err != nil {
		return err
	}

	err = installer.Install(ctx, cmdArgs, topoSorted, pkgBuildDirs)
	if err != nil {
		if errHook := installer.RunPostInstallHooks(ctx); errHook != nil {
			text.Errorln(errHook)
		}

		return err
	}

	return installer.RunPostInstallHooks(ctx)
}

type Preparer struct {
	dbExecutor db.Executor
	cmdBuilder exe.ICmdBuilder
	config     *settings.Configuration

	pkgBuildDirs []string
	makeDeps     []string
}

func (preper *Preparer) ShouldCleanMakeDeps(ctx context.Context) PostInstallHookFunc {
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
			pkgStr := text.Cyan(fmt.Sprintf("%s-%s", pkgName, info.Version))
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
	pkgBuildDirs := make(map[string]string, 0)

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
		preper.pkgBuildDirs, false, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}
	return pkgBuildDirs, nil
}

type (
	PostInstallHookFunc func(ctx context.Context) error
	Installer           struct {
		dbExecutor       db.Executor
		postInstallHooks []PostInstallHookFunc
	}
)

func (installer *Installer) AddPostInstallHook(hook PostInstallHookFunc) {
	installer.postInstallHooks = append(installer.postInstallHooks, hook)
}

func (Installer *Installer) RunPostInstallHooks(ctx context.Context) error {
	var errMulti multierror.MultiError
	for _, hook := range Installer.postInstallHooks {
		if err := hook(ctx); err != nil {
			errMulti.Add(err)
		}
	}
	return errMulti.Return()
}

func (installer *Installer) Install(ctx context.Context,
	cmdArgs *parser.Arguments,
	targets []map[string]*dep.InstallInfo,
	pkgBuildDirs map[string]string,
) error {
	// Reorganize targets into layers of dependencies
	for i := len(targets) - 1; i >= 0; i-- {
		err := installer.handleLayer(ctx, cmdArgs, targets[i], pkgBuildDirs)
		if err != nil {
			// rollback
			return err
		}
	}

	return nil
}

func (installer *Installer) handleLayer(ctx context.Context,
	cmdArgs *parser.Arguments, layer map[string]*dep.InstallInfo, pkgBuildDirs map[string]string,
) error {
	// Install layer
	nameToBaseMap := make(map[string]string, 0)
	syncDeps, syncExp := mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()
	aurDeps, aurExp := mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()
	for name, info := range layer {
		switch info.Source {
		case dep.SrcInfo:
			fallthrough
		case dep.AUR:
			nameToBaseMap[name] = *info.AURBase
			switch info.Reason {
			case dep.Explicit:
				if cmdArgs.ExistsArg("asdeps", "asdep") {
					aurDeps.Add(name)
				} else {
					aurExp.Add(name)
				}
			case dep.CheckDep:
				fallthrough
			case dep.MakeDep:
				fallthrough
			case dep.Dep:
				aurDeps.Add(name)
			}
		case dep.Sync:
			switch info.Reason {
			case dep.Explicit:
				if cmdArgs.ExistsArg("asdeps", "asdep") {
					syncDeps.Add(name)
				} else {
					syncExp.Add(name)
				}
			case dep.CheckDep:
				fallthrough
			case dep.MakeDep:
				fallthrough
			case dep.Dep:
				syncDeps.Add(name)
			}
		}
	}

	fmt.Println(syncDeps, syncExp)

	errShow := installer.installSyncPackages(ctx, cmdArgs, syncDeps, syncExp)
	if errShow != nil {
		return ErrInstallRepoPkgs
	}

	errAur := installer.installAURPackages(ctx, cmdArgs, aurDeps, aurExp, nameToBaseMap, pkgBuildDirs, false)

	return errAur
}

func (installer *Installer) installAURPackages(ctx context.Context,
	cmdArgs *parser.Arguments,
	aurDepNames, aurExpNames mapset.Set[string],
	nameToBase, pkgBuildDirsByBase map[string]string,
	installIncompatible bool,
) error {
	deps, exp := make([]string, 0, aurDepNames.Cardinality()), make([]string, 0, aurExpNames.Cardinality())

	for _, name := range aurDepNames.Union(aurExpNames).ToSlice() {
		base := nameToBase[name]
		dir := pkgBuildDirsByBase[base]
		args := []string{"--nobuild", "-fC"}

		if installIncompatible {
			args = append(args, "--ignorearch")
		}

		// pkgver bump
		if err := config.Runtime.CmdBuilder.Show(
			config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx, dir, args...)); err != nil {
			return errors.New(gotext.Get("error making: %s", base))
		}

		pkgdests, _, errList := parsePackageList(ctx, dir)
		if errList != nil {
			return errList
		}

		args = []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

		if installIncompatible {
			args = append(args, "--ignorearch")
		}

		if errMake := config.Runtime.CmdBuilder.Show(
			config.Runtime.CmdBuilder.BuildMakepkgCmd(ctx,
				dir, args...)); errMake != nil {
			return errors.New(gotext.Get("error making: %s", base))
		}

		names, err := installer.getNewTargets(pkgdests, name)
		if err != nil {
			return err
		}

		isDep := installer.isDep(cmdArgs, aurExpNames, name)

		if isDep {
			deps = append(deps, names...)
		} else {
			exp = append(exp, names...)
		}
	}

	if err := doInstall(ctx, cmdArgs, deps, exp); err != nil {
		return errors.New(fmt.Sprintf(gotext.Get("error installing:")+" %v %v", deps, exp))
	}

	return nil
}

func (*Installer) isDep(cmdArgs *parser.Arguments, aurExpNames mapset.Set[string], name string) bool {
	switch {
	case cmdArgs.ExistsArg("asdeps", "asdep"):
		return true
	case cmdArgs.ExistsArg("asexplicit", "asexp"):
		return false
	case aurExpNames.Contains(name):
		return false
	}

	return true
}

func (installer *Installer) getNewTargets(pkgdests map[string]string, name string,
) ([]string, error) {
	pkgdest, ok := pkgdests[name]
	names := make([]string, 0, 2)
	if !ok {
		return nil, errors.New(gotext.Get("could not find PKGDEST for: %s", name))
	}

	if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
		return nil, errors.New(
			gotext.Get(
				"the PKGDEST for %s is listed by makepkg but does not exist: %s",
				name, pkgdest))
	}

	names = append(names, name)

	debugName := pkgdest + "-debug"
	pkgdestDebug, ok := pkgdests[debugName]
	if ok {
		if _, errStat := os.Stat(pkgdestDebug); errStat == nil {
			names = append(names, debugName)
		}
	}

	return names, nil
}

func (*Installer) installSyncPackages(ctx context.Context, cmdArgs *parser.Arguments,
	syncDeps, // repo targets that are deps
	syncExp mapset.Set[string], // repo targets that are exp
) error {
	repoTargets := syncDeps.Union(syncExp).ToSlice()
	if len(repoTargets) == 0 {
		return nil
	}

	arguments := cmdArgs.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.DelArg("i", "install")
	arguments.Op = "S"
	arguments.ClearTargets()
	arguments.AddTarget(repoTargets...)

	errShow := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		arguments, config.Runtime.Mode, settings.NoConfirm))

	if errD := asdeps(ctx, cmdArgs, syncDeps.ToSlice()); errD != nil {
		return errD
	}

	if errE := asexp(ctx, cmdArgs, syncExp.ToSlice()); errE != nil {
		return errE
	}
	return errShow
}
