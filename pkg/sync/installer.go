package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"
)

type (
	PostInstallHookFunc func(ctx context.Context) error
	Installer           struct {
		dbExecutor       db.Executor
		postInstallHooks []PostInstallHookFunc
		failedAndIgnored map[string]error
		exeCmd           exe.ICmdBuilder
		vcsStore         vcs.Store
		targetMode       parser.TargetMode
		rebuildMode      parser.RebuildMode
		origTargets      mapset.Set[string]
		downloadOnly     bool
		log              *text.Logger

		manualConfirmRequired bool
	}
)

func NewInstaller(dbExecutor db.Executor,
	exeCmd exe.ICmdBuilder, vcsStore vcs.Store, targetMode parser.TargetMode,
	rebuildMode parser.RebuildMode, downloadOnly bool, logger *text.Logger,
) *Installer {
	return &Installer{
		dbExecutor:            dbExecutor,
		postInstallHooks:      []PostInstallHookFunc{},
		failedAndIgnored:      map[string]error{},
		exeCmd:                exeCmd,
		vcsStore:              vcsStore,
		targetMode:            targetMode,
		rebuildMode:           rebuildMode,
		downloadOnly:          downloadOnly,
		log:                   logger,
		manualConfirmRequired: true,
	}
}

func (installer *Installer) CompileFailedAndIgnored() (map[string]error, error) {
	if len(installer.failedAndIgnored) == 0 {
		return installer.failedAndIgnored, nil
	}

	return installer.failedAndIgnored, &FailedIgnoredPkgError{
		pkgErrors: installer.failedAndIgnored,
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
	excluded []string,
	manualConfirmRequired bool,
) error {
	installer.log.Debugln("manualConfirmRequired:", manualConfirmRequired)
	installer.manualConfirmRequired = manualConfirmRequired

	installer.origTargets = mapset.NewThreadUnsafeSet[string]()
	for _, targetString := range cmdArgs.Targets {
		installer.origTargets.Add(dep.ToTarget(targetString).Name)
	}
	installer.log.Debugln("origTargets:", installer.origTargets)

	// Reorganize targets into layers of dependencies
	var errMulti multierror.MultiError
	for i := len(targets) - 1; i >= 0; i-- {
		lastLayer := i == 0
		errI := installer.handleLayer(ctx, cmdArgs, targets[i], pkgBuildDirs, lastLayer, excluded)
		if errI == nil && lastLayer {
			// success after rollups
			return nil
		}

		if errI != nil {
			errMulti.Add(errI)
			if lastLayer {
				break
			}

			// rollup
			installer.log.Warnln(gotext.Get("Failed to install layer, rolling up to next layer."), "error:", errI)
			targets[i-1] = mergeLayers(targets[i-1], targets[i])
		}
	}

	return errMulti.Return()
}

func mergeLayers(layer1, layer2 map[string]*dep.InstallInfo) map[string]*dep.InstallInfo {
	for name, info := range layer2 {
		layer1[name] = info
	}

	return layer1
}

func (installer *Installer) appendNoConfirm() bool {
	return !installer.manualConfirmRequired || settings.NoConfirm
}

func (installer *Installer) handleLayer(ctx context.Context,
	cmdArgs *parser.Arguments,
	layer map[string]*dep.InstallInfo,
	pkgBuildDirs map[string]string,
	lastLayer bool,
	excluded []string,
) error {
	// Install layer
	nameToBaseMap := make(map[string]string, 0)
	syncDeps, syncExp, syncGroups := mapset.NewThreadUnsafeSet[string](),
		mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()
	aurDeps, aurExp := mapset.NewThreadUnsafeSet[string](), mapset.NewThreadUnsafeSet[string]()

	upgradeSync := false
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
			if info.Upgrade {
				upgradeSync = true
				continue // do not add to targets, let pacman handle it
			}
			compositePkgName := fmt.Sprintf("%s/%s", *info.SyncDBName, name)

			if info.IsGroup {
				syncGroups.Add(compositePkgName)
				continue
			}

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

	installer.log.Debugln("syncDeps", syncDeps, "SyncExp", syncExp,
		"aurDeps", aurDeps, "aurExp", aurExp, "upgrade", upgradeSync)

	errShow := installer.installSyncPackages(ctx, cmdArgs, syncDeps, syncExp, syncGroups,
		excluded, upgradeSync, installer.appendNoConfirm())
	if errShow != nil {
		return ErrInstallRepoPkgs
	}

	errAur := installer.installAURPackages(ctx, cmdArgs, aurDeps, aurExp,
		nameToBaseMap, pkgBuildDirs, true, lastLayer, installer.appendNoConfirm())

	return errAur
}

func (installer *Installer) installAURPackages(ctx context.Context,
	cmdArgs *parser.Arguments,
	aurDepNames, aurExpNames mapset.Set[string],
	nameToBase, pkgBuildDirsByBase map[string]string,
	installIncompatible bool,
	lastLayer bool,
	noConfirm bool,
) error {
	all := aurDepNames.Union(aurExpNames).ToSlice()
	if len(all) == 0 {
		return nil
	}

	deps, exps := make([]string, 0, aurDepNames.Cardinality()), make([]string, 0, aurExpNames.Cardinality())
	pkgArchives := make([]string, 0, len(exps)+len(deps))

	for _, name := range all {
		base := nameToBase[name]
		dir := pkgBuildDirsByBase[base]

		pkgdests, errMake := installer.buildPkg(ctx, dir, base,
			installIncompatible, cmdArgs.ExistsArg("needed"), installer.origTargets.Contains(name))
		if errMake != nil {
			if !lastLayer {
				return fmt.Errorf("%s - %w", gotext.Get("error making: %s", base), errMake)
			}

			installer.failedAndIgnored[name] = errMake
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
	}

	if err := installPkgArchive(ctx, installer.exeCmd, installer.targetMode,
		installer.vcsStore, cmdArgs, pkgArchives, noConfirm); err != nil {
		return fmt.Errorf("%s - %w", fmt.Sprintf(gotext.Get("error installing:")+" %v", pkgArchives), err)
	}

	if err := setInstallReason(ctx, installer.exeCmd, installer.targetMode, cmdArgs, deps, exps); err != nil {
		return fmt.Errorf("%s - %w", fmt.Sprintf(gotext.Get("error installing:")+" %v", pkgArchives), err)
	}

	return nil
}

func (installer *Installer) buildPkg(ctx context.Context,
	dir, base string,
	installIncompatible, needed, isTarget bool,
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
	case needed && installer.pkgsAreAlreadyInstalled(pkgdests, pkgVersion) || installer.downloadOnly:
		args = []string{"-c", "--nobuild", "--noextract", "--ignorearch"}
		pkgdests = map[string]string{}
		text.Warnln(gotext.Get("%s is up to date -- skipping", text.Cyan(base+"-"+pkgVersion)))
	case installer.skipAlreadyBuiltPkg(isTarget, pkgdests):
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

	if installer.downloadOnly {
		return map[string]string{}, nil
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

func (installer *Installer) skipAlreadyBuiltPkg(isTarget bool, pkgdests map[string]string) bool {
	switch installer.rebuildMode {
	case parser.RebuildModeNo:
		return pkgsAreBuilt(pkgdests)
	case parser.RebuildModeYes:
		return !isTarget && pkgsAreBuilt(pkgdests)
	// case parser.RebuildModeTree: // TODO
	// case parser.RebuildModeAll: // TODO
	default:
		// same as RebuildModeNo
		return pkgsAreBuilt(pkgdests)
	}
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

	debugName := name + "-debug"

	pkgdestDebug, ok := pkgdests[debugName]
	if ok {
		if _, errStat := os.Stat(pkgdestDebug); errStat == nil {
			pkgArchives = append(pkgArchives, pkgdestDebug)
		} else {
			ok = false
		}
	}

	return pkgArchives, ok, nil
}

func (installer *Installer) installSyncPackages(ctx context.Context, cmdArgs *parser.Arguments,
	syncDeps, // repo targets that are deps
	syncExp mapset.Set[string], // repo targets that are exp
	syncGroups mapset.Set[string], // repo targets that are groups
	excluded []string,
	upgrade bool, // run even without targets
	noConfirm bool,
) error {
	repoTargets := syncDeps.Union(syncExp).Union(syncGroups).ToSlice()
	if len(repoTargets) == 0 && !upgrade {
		return nil
	}

	arguments := cmdArgs.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.DelArg("i", "install")
	arguments.Op = "S"
	arguments.ClearTargets()
	arguments.AddTarget(repoTargets...)

	// Don't upgrade all repo packages if only AUR upgrades are specified
	if installer.targetMode == parser.ModeAUR {
		arguments.DelArg("u", "upgrades")
	}

	if len(excluded) > 0 {
		arguments.CreateOrAppendOption("ignore", excluded...)
	}

	errShow := installer.exeCmd.Show(installer.exeCmd.BuildPacmanCmd(ctx,
		arguments, installer.targetMode, noConfirm))
	if errShow != nil {
		return errShow
	}

	if errD := asdeps(ctx, installer.exeCmd, installer.targetMode, cmdArgs, syncDeps.ToSlice()); errD != nil {
		return errD
	}

	if errE := asexp(ctx, installer.exeCmd, installer.targetMode, cmdArgs, syncExp.ToSlice()); errE != nil {
		return errE
	}

	return nil
}
