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

	if err := preparer.PrepareWorkspace(ctx, topoSorted); err != nil {
		return err
	}

	return installer.Install(ctx, cmdArgs, topoSorted)
}

type Preparer struct {
	dbExecutor   db.Executor
	cmdBuilder   exe.ICmdBuilder
	aurBases     []string
	pkgBuildDirs []string
}

func (preper *Preparer) PrepareWorkspace(ctx context.Context, targets []map[string]*dep.InstallInfo,
) error {
	for _, layer := range targets {
		for pkgBase, info := range layer {
			if info.Source == dep.AUR {
				preper.aurBases = append(preper.aurBases, pkgBase)
				preper.pkgBuildDirs = append(preper.pkgBuildDirs, filepath.Join(config.BuildDir, pkgBase))
			} else if info.Source == dep.SrcInfo {
				preper.pkgBuildDirs = append(preper.pkgBuildDirs, *info.SrcinfoPath)
			}
		}
	}

	if _, errA := download.AURPKGBUILDRepos(ctx,
		preper.cmdBuilder, preper.aurBases, config.AURURL, config.BuildDir, false); errA != nil {
		return errA
	}

	if errP := downloadPKGBUILDSourceFanout(ctx, config.Runtime.CmdBuilder,
		preper.pkgBuildDirs, false, config.MaxConcurrentDownloads); errP != nil {
		text.Errorln(errP)
	}
	return nil
}

type Installer struct {
	dbExecutor db.Executor
}

func (installer *Installer) Install(ctx context.Context, cmdArgs *parser.Arguments, targets []map[string]*dep.InstallInfo) error {
	// Reorganize targets into layers of dependencies
	for i := len(targets) - 1; i >= 0; i-- {
		err := installer.handleLayer(ctx, cmdArgs, targets[i])
		if err != nil {
			// rollback
			return err
		}
	}

	return nil
}

type MapBySourceAndType map[dep.Source]map[dep.Reason][]string

func (m *MapBySourceAndType) String() string {
	var s string
	for source, reasons := range *m {
		s += fmt.Sprintf("%s: [", source)
		for reason, names := range reasons {
			s += fmt.Sprintf(" %d: [%v] ", reason, names)
		}

		s += "], "
	}

	return s
}

func (installer *Installer) handleLayer(ctx context.Context, cmdArgs *parser.Arguments, layer map[string]*dep.InstallInfo) error {
	// Install layer
	depByTypeAndReason := make(MapBySourceAndType)
	for name, info := range layer {
		if _, ok := depByTypeAndReason[info.Source]; !ok {
			depByTypeAndReason[info.Source] = make(map[dep.Reason][]string)
		}

		depByTypeAndReason[info.Source][info.Reason] = append(depByTypeAndReason[info.Source][info.Reason], name)
	}

	fmt.Printf("%v\n", depByTypeAndReason)

	syncDeps, syncExp := make([]string, 0), make([]string, 0)
	repoTargets := make([]string, 0)

	aurDeps, aurExp := mapset.NewSet[string](), mapset.NewSet[string]()
	for source, reasons := range depByTypeAndReason {
		switch source {
		case dep.AUR:
			for reason, names := range reasons {
				for _, name := range names {
					switch reason {
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
				}
			}
		case dep.Sync:
			for reason, names := range reasons {
				switch reason {
				case dep.Explicit:
					if cmdArgs.ExistsArg("asdeps", "asdep") {
						syncDeps = append(syncDeps, names...)
					} else {
						syncExp = append(syncExp, names...)
					}
				case dep.CheckDep:
					fallthrough
				case dep.MakeDep:
					fallthrough
				case dep.Dep:
					syncDeps = append(syncDeps, names...)
				}

				repoTargets = append(repoTargets, names...)
			}
		}
	}

	fmt.Println(syncDeps, syncExp)

	errShow := installer.installSyncPackages(ctx, cmdArgs, repoTargets, syncDeps, syncExp)
	if errShow != nil {
		return ErrInstallRepoPkgs
	}

	return nil
}

func (*Installer) installAURPackages(ctx context.Context, cmdArgs *parser.Arguments, aurBaseDeps, aurBaseExp mapset.Set[string], pkgBuildDirs map[string]string, installIncompatible bool) error {
	deps, exp := make([]string, 0, aurBaseDeps.Cardinality()), make([]string, 0, aurBaseExp.Cardinality())
	for _, base := range aurBaseDeps.Union(aurBaseExp).ToSlice() {
		dir := pkgBuildDirs[base]
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

		for suffix, optional := range map[string]bool{"": false, "-debug": true} {
			newDeps, newExp, err := getNewTargets(cmdArgs, pkgdests, base+suffix, aurBaseDeps.Contains(base), optional)
			if err != nil {
				return err
			}
			deps = append(deps, newDeps...)
			exp = append(exp, newExp...)
		}
	}

	if err := doInstall(ctx, cmdArgs, deps, exp); err != nil {
		return errors.New(fmt.Sprintf(gotext.Get("error installing:")+" %v %v", deps, exp))
	}

	return nil
}

func getNewTargets(cmdArgs *parser.Arguments, pkgdests map[string]string, name string, isDep, optional bool,
) (deps, exp []string, err error) {
	for pkgName, pkgDest := range pkgdests {
		if _, errStat := os.Stat(pkgDest); os.IsNotExist(errStat) {
			if optional {
				continue
			}

			return deps, exp, errors.New(
				gotext.Get(
					"the PKGDEST for %s is listed by makepkg but does not exist: %s",
					pkgName, pkgDest))
		}

		switch {
		case cmdArgs.ExistsArg("asdeps", "asdep"):
			deps = append(deps, name)
		case cmdArgs.ExistsArg("asexplicit", "asexp"):
			exp = append(exp, name)
		case isDep:
			deps = append(deps, name)
		default:
			exp = append(exp, name)
		}
	}

	return deps, exp, nil
}

func (*Installer) installSyncPackages(ctx context.Context, cmdArgs *parser.Arguments,
	repoTargets, // all repo targets
	syncDeps, // repo targets that are deps
	syncExp []string, // repo targets that are exp
) error {
	arguments := cmdArgs.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.DelArg("i", "install")
	arguments.Op = "S"
	arguments.ClearTargets()
	arguments.AddTarget(repoTargets...)

	errShow := config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
		arguments, config.Runtime.Mode, settings.NoConfirm))

	if errD := asdeps(ctx, cmdArgs, syncDeps); errD != nil {
		return errD
	}

	if errE := asexp(ctx, cmdArgs, syncExp); errE != nil {
		return errE
	}
	return errShow
}
