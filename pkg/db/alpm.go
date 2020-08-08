package db

import (
	"errors"

	alpm "github.com/Jguer/go-alpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/upgrade"
)

type AlpmExecutor struct {
	handle           *alpm.Handle
	localDB          *alpm.DB
	syncDB           alpm.DBList
	conf             *pacmanconf.Config
	questionCallback func(question alpm.QuestionAny)
}

func NewAlpmExecutor(handle *alpm.Handle,
	pacamnConf *pacmanconf.Config,
	questionCallback func(question alpm.QuestionAny)) (*AlpmExecutor, error) {
	localDB, err := handle.LocalDB()
	if err != nil {
		return nil, err
	}
	syncDB, err := handle.SyncDBs()
	if err != nil {
		return nil, err
	}

	return &AlpmExecutor{handle: handle, localDB: localDB, syncDB: syncDB, conf: pacamnConf, questionCallback: questionCallback}, nil
}

func toUsage(usages []string) alpm.Usage {
	if len(usages) == 0 {
		return alpm.UsageAll
	}

	var ret alpm.Usage
	for _, usage := range usages {
		switch usage {
		case "Sync":
			ret |= alpm.UsageSync
		case "Search":
			ret |= alpm.UsageSearch
		case "Install":
			ret |= alpm.UsageInstall
		case "Upgrade":
			ret |= alpm.UsageUpgrade
		case "All":
			ret |= alpm.UsageAll
		}
	}

	return ret
}

func configureAlpm(pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle) error {
	// TODO: set SigLevel
	// sigLevel := alpm.SigPackage | alpm.SigPackageOptional | alpm.SigDatabase | alpm.SigDatabaseOptional
	// localFileSigLevel := alpm.SigUseDefault
	// remoteFileSigLevel := alpm.SigUseDefault

	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		db, err := alpmHandle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return err
		}

		db.SetServers(repo.Servers)
		db.SetUsage(toUsage(repo.Usage))
	}

	if err := alpmHandle.SetCacheDirs(pacmanConf.CacheDir); err != nil {
		return err
	}

	// add hook directories 1-by-1 to avoid overwriting the system directory
	for _, dir := range pacmanConf.HookDir {
		if err := alpmHandle.AddHookDir(dir); err != nil {
			return err
		}
	}

	if err := alpmHandle.SetGPGDir(pacmanConf.GPGDir); err != nil {
		return err
	}

	if err := alpmHandle.SetLogFile(pacmanConf.LogFile); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnorePkgs(pacmanConf.IgnorePkg); err != nil {
		return err
	}

	if err := alpmHandle.SetIgnoreGroups(pacmanConf.IgnoreGroup); err != nil {
		return err
	}

	if err := alpmHandle.SetArch(pacmanConf.Architecture); err != nil {
		return err
	}

	if err := alpmHandle.SetNoUpgrades(pacmanConf.NoUpgrade); err != nil {
		return err
	}

	if err := alpmHandle.SetNoExtracts(pacmanConf.NoExtract); err != nil {
		return err
	}

	/*if err := alpmHandle.SetDefaultSigLevel(sigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetLocalFileSigLevel(localFileSigLevel); err != nil {
		return err
	}

	if err := alpmHandle.SetRemoteFileSigLevel(remoteFileSigLevel); err != nil {
		return err
	}*/

	if err := alpmHandle.SetUseSyslog(pacmanConf.UseSyslog); err != nil {
		return err
	}

	return alpmHandle.SetCheckSpace(pacmanConf.CheckSpace)
}

func logCallback(level alpm.LogLevel, str string) {
	switch level {
	case alpm.LogWarning:
		text.Warn(str)
	case alpm.LogError:
		text.Error(str)
	}
}

func (ae *AlpmExecutor) RefreshHandle() error {
	if ae.handle != nil {
		if errRelease := ae.handle.Release(); errRelease != nil {
			return errRelease
		}
	}

	alpmHandle, err := alpm.Initialize(ae.conf.RootDir, ae.conf.DBPath)
	if err != nil {
		return errors.New(gotext.Get("unable to CreateHandle: %s", err))
	}

	if errConf := configureAlpm(ae.conf, alpmHandle); errConf != nil {
		return errConf
	}

	alpmHandle.SetQuestionCallback(ae.questionCallback)
	alpmHandle.SetLogCallback(logCallback)
	ae.handle = alpmHandle
	ae.syncDB, err = alpmHandle.SyncDBs()
	if err != nil {
		return err
	}

	ae.localDB, err = alpmHandle.LocalDB()
	return err
}

func (ae *AlpmExecutor) LocalSatisfierExists(pkgName string) bool {
	if _, err := ae.localDB.PkgCache().FindSatisfier(pkgName); err != nil {
		return false
	}
	return true
}

func (ae *AlpmExecutor) SyncSatisfierExists(pkgName string) bool {
	if _, err := ae.syncDB.FindSatisfier(pkgName); err != nil {
		return false
	}
	return true
}

func (ae *AlpmExecutor) IsCorrectVersionInstalled(pkgName, versionRequired string) bool {
	alpmPackage := ae.localDB.Pkg(pkgName)
	if alpmPackage == nil {
		return false
	}

	return alpmPackage.Version() == versionRequired
}

func (ae *AlpmExecutor) SyncSatisfier(pkgName string) RepoPackage {
	foundPkg, err := ae.syncDB.FindSatisfier(pkgName)
	if err != nil {
		return nil
	}
	return foundPkg
}

func (ae *AlpmExecutor) PackagesFromGroup(groupName string) []RepoPackage {
	groupPackages := []RepoPackage{}
	_ = ae.syncDB.FindGroupPkgs(groupName).ForEach(func(pkg alpm.Package) error {
		groupPackages = append(groupPackages, &pkg)
		return nil
	})
	return groupPackages
}

func (ae *AlpmExecutor) LocalPackages() []RepoPackage {
	localPackages := []RepoPackage{}
	_ = ae.localDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		localPackages = append(localPackages, RepoPackage(&pkg))
		return nil
	})
	return localPackages
}

// SyncPackages searches SyncDB for packages or returns all packages if no search param is given
func (ae *AlpmExecutor) SyncPackages(pkgNames ...string) []RepoPackage {
	repoPackages := []RepoPackage{}
	_ = ae.syncDB.ForEach(func(db alpm.DB) error {
		if len(pkgNames) == 0 {
			_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
				repoPackages = append(repoPackages, RepoPackage(&pkg))
				return nil
			})
		} else {
			_ = db.Search(pkgNames).ForEach(func(pkg alpm.Package) error {
				repoPackages = append(repoPackages, RepoPackage(&pkg))
				return nil
			})
		}
		return nil
	})
	return repoPackages
}

func (ae *AlpmExecutor) LocalPackage(pkgName string) RepoPackage {
	pkg := ae.localDB.Pkg(pkgName)
	if pkg == nil {
		return nil
	}
	return pkg
}

func (ae *AlpmExecutor) PackageFromDB(pkgName, dbName string) RepoPackage {
	singleDB, err := ae.handle.SyncDBByName(dbName)
	if err != nil {
		return nil
	}
	foundPkg, err := singleDB.PkgCache().FindSatisfier(pkgName)
	if err != nil {
		return nil
	}
	return foundPkg
}

func (ae *AlpmExecutor) PackageDepends(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Depends().Slice()
}

func (ae *AlpmExecutor) PackageOptionalDepends(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.OptionalDepends().Slice()
}

func (ae *AlpmExecutor) PackageProvides(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Provides().Slice()
}

func (ae *AlpmExecutor) PackageConflicts(pkg RepoPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Conflicts().Slice()
}

func (ae *AlpmExecutor) PackageGroups(pkg RepoPackage) []string {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Groups().Slice()
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func (ae *AlpmExecutor) RepoUpgrades(enableDowngrade bool) (upgrade.UpSlice, error) {
	slice := upgrade.UpSlice{}

	localDB, err := ae.handle.LocalDB()
	if err != nil {
		return slice, err
	}

	err = ae.handle.TransInit(alpm.TransFlagNoLock)
	if err != nil {
		return slice, err
	}

	defer func() {
		err = ae.handle.TransRelease()
	}()

	err = ae.handle.SyncSysupgrade(enableDowngrade)
	if err != nil {
		return slice, err
	}
	_ = ae.handle.TransGetAdd().ForEach(func(pkg alpm.Package) error {
		localVer := "-"

		if localPkg := localDB.Pkg(pkg.Name()); localPkg != nil {
			localVer = localPkg.Version()
		}

		slice = append(slice, upgrade.Upgrade{
			Name:          pkg.Name(),
			Repository:    pkg.DB().Name(),
			LocalVersion:  localVer,
			RemoteVersion: pkg.Version(),
		})
		return nil
	})

	return slice, nil
}

func (ae *AlpmExecutor) AlpmArch() (string, error) {
	return ae.handle.Arch()
}

func (ae *AlpmExecutor) BiggestPackages() []RepoPackage {
	localPackages := []RepoPackage{}
	_ = ae.localDB.PkgCache().SortBySize().ForEach(func(pkg alpm.Package) error {
		localPackages = append(localPackages, RepoPackage(&pkg))
		return nil
	})
	return localPackages
}
