package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/vcs"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type (
	PostInstallHookFunc func(ctx context.Context) error
	Installer           struct {
		dbExecutor        db.Executor
		postInstallHooks  []PostInstallHookFunc
		failedAndIngnored map[string]error
		exeCmd            exe.ICmdBuilder
		vcsStore          vcs.Store
		targetMode        parser.TargetMode
	}
)

func NewInstaller(dbExecutor db.Executor, exeCmd exe.ICmdBuilder, vcsStore vcs.Store, targetMode parser.TargetMode) *Installer {
	return &Installer{
		dbExecutor:        dbExecutor,
		postInstallHooks:  []PostInstallHookFunc{},
		failedAndIngnored: map[string]error{},
		exeCmd:            exeCmd,
		vcsStore:          vcsStore,
		targetMode:        targetMode,
	}
}

func (installer *Installer) CompileFailedAndIgnored() error {
	if len(installer.failedAndIngnored) == 0 {
		return nil
	}

	return &FailedIgnoredPkgError{
		pkgErrors: installer.failedAndIngnored,
	}
}

func (installer *Installer) AddPostInstallHook(hook PostInstallHookFunc) {
	if hook == nil {
		return
	}

	installer.postInstallHooks = append(installer.postInstallHooks, hook)
}

func (installer *Installer) RunPostInstallHooks(ctx context.Context) error {
	var errMulti multierror.MultiError

	for _, hook := range installer.postInstallHooks {
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
	srcinfos map[string]*gosrc.Srcinfo,
) error {
	// Reorganize targets into layers of dependencies
	for i := len(targets) - 1; i >= 0; i-- {
		err := installer.handleLayer(ctx, cmdArgs, targets[i], pkgBuildDirs, srcinfos, i == 0)
		if err != nil {
			// rollback
			return err
		}
	}

	return nil
}

func (installer *Installer) handleLayer(ctx context.Context,
	cmdArgs *parser.Arguments,
	layer map[string]*dep.InstallInfo,
	pkgBuildDirs map[string]string,
	srcinfos map[string]*gosrc.Srcinfo,
	lastLayer bool,
) error {
	// Install layer
	nameToBaseMap := make(map[string]string, 0)
	syncDeps, syncExp := mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()
	aurDeps, aurExp := mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()

	for name, info := range layer {
		switch info.Source {
		case dep.AUR, dep.SrcInfo:
			nameToBaseMap[name] = *info.AURBase

			switch info.Reason {
			case dep.Explicit:
				if cmdArgs.ExistsArg("asdeps", "asdep") {
					aurDeps.Add(name)
				} else {
					aurExp.Add(name)
				}
			case dep.Dep, dep.MakeDep, dep.CheckDep:
				aurDeps.Add(name)
			}
		case dep.Sync:
			compositePkgName := fmt.Sprintf("%s/%s", *info.SyncDBName, name)

			switch info.Reason {
			case dep.Explicit:
				if cmdArgs.ExistsArg("asdeps", "asdep") {
					syncDeps.Add(compositePkgName)
				} else {
					syncExp.Add(compositePkgName)
				}
			case dep.Dep, dep.MakeDep, dep.CheckDep:
				syncDeps.Add(compositePkgName)
			}
		}
	}

	text.Debugln("syncDeps", syncDeps, "SyncExp", syncExp, "aurDeps", aurDeps, "aurExp", aurExp)

	errShow := installer.installSyncPackages(ctx, cmdArgs, syncDeps, syncExp)
	if errShow != nil {
		return ErrInstallRepoPkgs
	}

	errAur := installer.installAURPackages(ctx, cmdArgs, aurDeps, aurExp,
		nameToBaseMap, pkgBuildDirs, true, srcinfos, lastLayer)

	return errAur
}

func (installer *Installer) installAURPackages(ctx context.Context,
	cmdArgs *parser.Arguments,
	aurDepNames, aurExpNames mapset.Set[string],
	nameToBase, pkgBuildDirsByBase map[string]string,
	installIncompatible bool,
	srcinfos map[string]*gosrc.Srcinfo,
	lastLayer bool,
) error {
	all := aurDepNames.Union(aurExpNames).ToSlice()
	if len(all) == 0 {
		return nil
	}

	deps, exps := make([]string, 0, aurDepNames.Cardinality()), make([]string, 0, aurExpNames.Cardinality())
	pkgArchives := make([]string, 0, len(exps)+len(deps))

	var wg sync.WaitGroup

	for _, name := range all {
		base := nameToBase[name]
		dir := pkgBuildDirsByBase[base]

		pkgdests, errMake := installer.buildPkg(ctx, dir, base, installIncompatible, cmdArgs.ExistsArg("needed"))
		if errMake != nil {
			if !lastLayer {
				return fmt.Errorf("%s - %w", gotext.Get("error making: %s", base), errMake)
			}

			installer.failedAndIngnored[name] = errMake
			text.Errorln(gotext.Get("error making: %s", base), "-", errMake)
			continue
		}

		if len(pkgdests) == 0 {
			text.Warnln(gotext.Get("nothing to install for %s", text.Cyan(base)))
			continue
		}

		newPKGArchives, hasDebug, err := installer.getNewTargets(pkgdests, name)
		if err != nil {
			return err
		}

		pkgArchives = append(pkgArchives, newPKGArchives...)

		if isDep := installer.isDep(cmdArgs, aurExpNames, name); isDep {
			deps = append(deps, name)
		} else {
			exps = append(exps, name)
		}

		if hasDebug {
			deps = append(deps, name+"-debug")
		}

		srcinfo := srcinfos[base]
		wg.Add(1)
		go func(name string) {
			installer.vcsStore.Update(ctx, name, srcinfo.Source)
			wg.Done()
		}(name)
	}

	wg.Wait()

	if err := installPkgArchive(ctx, installer.exeCmd, installer.targetMode, installer.vcsStore, cmdArgs, pkgArchives); err != nil {
		return fmt.Errorf("%s - %w", fmt.Sprintf(gotext.Get("error installing:")+" %v", pkgArchives), err)
	}

	if err := setInstallReason(ctx, installer.exeCmd, installer.targetMode, cmdArgs, deps, exps); err != nil {
		return fmt.Errorf("%s - %w", fmt.Sprintf(gotext.Get("error installing:")+" %v", pkgArchives), err)
	}

	return nil
}

func (installer *Installer) buildPkg(ctx context.Context,
	dir, base string,
	installIncompatible, needed bool,
) (map[string]string, error) {
	args := []string{"--nobuild", "-fC"}

	if installIncompatible {
		args = append(args, "--ignorearch")
	}

	// pkgver bump
	if err := installer.exeCmd.Show(
		installer.exeCmd.BuildMakepkgCmd(ctx, dir, args...)); err != nil {
		return nil, err
	}

	pkgdests, pkgVersion, errList := parsePackageList(ctx, installer.exeCmd, dir)
	if errList != nil {
		return nil, errList
	}

	switch {
	case needed && installer.pkgsAreAlreadyInstalled(pkgdests, pkgVersion):
		args = []string{"-c", "--nobuild", "--noextract", "--ignorearch"}
		pkgdests = map[string]string{}
		text.Warnln(gotext.Get("%s is up to date -- skipping", text.Cyan(base+"-"+pkgVersion)))
	case pkgsAreBuilt(pkgdests):
		args = []string{"-c", "--nobuild", "--noextract", "--ignorearch"}
		text.Warnln(gotext.Get("%s already made -- skipping build", text.Cyan(base+"-"+pkgVersion)))
	default:
		args = []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}
		if installIncompatible {
			args = append(args, "--ignorearch")
		}
	}

	errMake := installer.exeCmd.Show(
		installer.exeCmd.BuildMakepkgCmd(ctx,
			dir, args...))
	if errMake != nil {
		return nil, errMake
	}

	return pkgdests, nil
}

func (installer *Installer) pkgsAreAlreadyInstalled(pkgdests map[string]string, pkgVersion string) bool {
	for pkgName := range pkgdests {
		if !installer.dbExecutor.IsCorrectVersionInstalled(pkgName, pkgVersion) {
			return false
		}
	}

	return true
}

func pkgsAreBuilt(pkgdests map[string]string) bool {
	for _, pkgdest := range pkgdests {
		if _, err := os.Stat(pkgdest); err != nil {
			text.Debugln("pkgIsBuilt:", pkgdest, "does not exist")
			return false
		}
	}

	return true
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
) (archives []string, good bool, err error) {
	pkgdest, ok := pkgdests[name]
	if !ok {
		return nil, false, &PkgDestNotInListError{name: name}
	}

	pkgArchives := make([]string, 0, 2)

	if _, errStat := os.Stat(pkgdest); os.IsNotExist(errStat) {
		return nil, false, &FindPkgDestError{name: name, pkgDest: pkgdest}
	}

	pkgArchives = append(pkgArchives, pkgdest)

	debugName := pkgdest + "-debug"

	pkgdestDebug, ok := pkgdests[debugName]
	if ok {
		if _, errStat := os.Stat(pkgdestDebug); errStat == nil {
			pkgArchives = append(pkgArchives, debugName)
		}
	}

	return pkgArchives, ok, nil
}

func (installer *Installer) installSyncPackages(ctx context.Context, cmdArgs *parser.Arguments,
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
	arguments.DelArg("u", "upgrade")
	arguments.Op = "S"
	arguments.ClearTargets()
	arguments.AddTarget(repoTargets...)

	errShow := installer.exeCmd.Show(installer.exeCmd.BuildPacmanCmd(ctx,
		arguments, installer.targetMode, settings.NoConfirm))

	if errD := asdeps(ctx, installer.exeCmd, installer.targetMode, cmdArgs, syncDeps.ToSlice()); errD != nil {
		return errD
	}

	if errE := asexp(ctx, installer.exeCmd, installer.targetMode, cmdArgs, syncExp.ToSlice()); errE != nil {
		return errE
	}

	return errShow
}
