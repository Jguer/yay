// Experimental code for install local with dependency refactoring
// Not at feature parity with install.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/download"
	"github.com/Jguer/yay/v11/pkg/metadata"
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

	preparer := &Preparer{dbExecutor: dbExecutor, cmdBuilder: config.Runtime.CmdBuilder}
	installer := &Installer{dbExecutor: dbExecutor}

	pkgBuildDirs, err := preparer.PrepareWorkspace(ctx, topoSorted)
	if err != nil {
		return err
	}

	return installer.Install(ctx, cmdArgs, topoSorted, pkgBuildDirs)
}

type Preparer struct {
	dbExecutor   db.Executor
	cmdBuilder   exe.ICmdBuilder
	aurBases     []string
	pkgBuildDirs []string
}

func (preper *Preparer) PrepareWorkspace(ctx context.Context, targets []map[string]*dep.InstallInfo,
) (map[string]string, error) {
	pkgBuildDirs := make(map[string]string, 0)

	for _, layer := range targets {
		for pkgBase, info := range layer {
			if info.Source == dep.AUR {
				preper.aurBases = append(preper.aurBases, pkgBase)
				pkgBuildDirs[pkgBase] = filepath.Join(config.BuildDir, pkgBase)
			} else if info.Source == dep.SrcInfo {
				pkgBuildDirs[pkgBase] = *info.SrcinfoPath
			}
		}
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		preper.cmdBuilder, preper.aurBases, config.AURURL, config.BuildDir, false); errA != nil {
		return nil, errA
	}

	if errP := downloadPKGBUILDSourceFanout(ctx, config.Runtime.CmdBuilder,
		preper.pkgBuildDirs, false, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}
	return pkgBuildDirs, nil
}

type Installer struct {
	dbExecutor db.Executor
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
