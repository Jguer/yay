package ialpm

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	alpm "github.com/Jguer/dyalpm"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
)

type AlpmExecutor struct {
	handle       alpm.Handle
	localDB      alpm.Database
	syncDB       []alpm.Database
	syncDBsCache []alpm.Database
	conf         *pacmanconf.Config
	log          *text.Logger

	installedRemotePkgNames []string
	installedRemotePkgMap   map[string]alpm.Package
	installedSyncPkgNames   []string
}

func NewExecutor(pacmanConf *pacmanconf.Config, logger *text.Logger) (*AlpmExecutor, error) {
	ae := &AlpmExecutor{
		handle:                  nil,
		localDB:                 nil,
		syncDB:                  nil,
		syncDBsCache:            []alpm.Database{},
		conf:                    pacmanConf,
		log:                     logger,
		installedRemotePkgNames: nil,
		installedRemotePkgMap:   nil,
		installedSyncPkgNames:   nil,
	}

	if err := ae.RefreshHandle(); err != nil {
		return nil, err
	}

	var err error
	ae.localDB, err = ae.handle.LocalDB()
	if err != nil {
		return nil, err
	}

	ae.syncDB, err = ae.handle.SyncDBs()
	if err != nil {
		return nil, err
	}

	return ae, nil
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

func configureAlpm(pacmanConf *pacmanconf.Config, alpmHandle alpm.Handle) error {
	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		alpmDB, err := alpmHandle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return err
		}

		if err := alpmDB.SetServers(repo.Servers); err != nil {
			return err
		}
		if err := alpmDB.SetUsage(int(toUsage(repo.Usage))); err != nil {
			return err
		}
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

	if err := alpmSetArchitecture(alpmHandle, pacmanConf.Architecture); err != nil {
		return err
	}

	if err := alpmHandle.SetNoUpgrades(pacmanConf.NoUpgrade); err != nil {
		return err
	}

	if err := alpmHandle.SetNoExtracts(pacmanConf.NoExtract); err != nil {
		return err
	}

	if err := alpmHandle.SetUseSyslog(pacmanConf.UseSyslog); err != nil {
		return err
	}

	return alpmHandle.SetCheckSpace(pacmanConf.CheckSpace)
}

func (ae *AlpmExecutor) logCallback() func(level alpm.LogLevel, str string) {
	return func(level alpm.LogLevel, str string) {
		switch level {
		case alpm.LogWarning:
			ae.log.Warn(str)
		case alpm.LogError:
			ae.log.Error(str)
		}
	}
}

func (ae *AlpmExecutor) questionCallback() func(question alpm.QuestionAny) {
	return func(question alpm.QuestionAny) {
		if qi, err := question.QuestionInstallIgnorepkg(); err == nil {
			qi.SetInstall(true)
		}

		qp, err := question.QuestionSelectProvider()
		if err != nil {
			return
		}

		if settings.HideMenus {
			return
		}

		size := 0

		_ = qp.Providers(ae.handle).ForEach(func(pkg alpm.Package) error {
			size++
			return nil
		})

		str := text.Bold(gotext.Get("There are %[1]d providers available for %[2]s:", size, qp.Dep()))

		size = 1

		var dbName string

		_ = qp.Providers(ae.handle).ForEach(func(pkg alpm.Package) error {
			thisDB := pkg.DB().Name()

			if dbName != thisDB {
				dbName = thisDB
				str += "\n"
				str += ae.log.SprintOperationInfo(gotext.Get("Repository"), " ", dbName, "\n    ")
			}
			str += fmt.Sprintf("%d) %s ", size, pkg.Name())
			size++
			return nil
		})

		ae.log.OperationInfoln(str)

		for {
			ae.log.Println(gotext.Get("\nEnter a number (default=1): "))

			// TODO: reenable noconfirm
			if settings.NoConfirm {
				ae.log.Println()

				break
			}

			numberBuf, err := ae.log.GetInput("", false)
			if err != nil {
				ae.log.Errorln(err)
				break
			}

			if numberBuf == "" {
				break
			}

			num, err := strconv.Atoi(numberBuf)
			if err != nil {
				ae.log.Errorln(gotext.Get("invalid number: %s", numberBuf))
				continue
			}

			if num < 1 || num > size {
				ae.log.Errorln(gotext.Get("invalid value: %d is not between %d and %d", num, 1, size))
				continue
			}

			qp.SetUseIndex(num - 1)

			break
		}
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

	if err := alpmSetQuestionCallback(alpmHandle, ae.questionCallback()); err != nil {
		return err
	}
	alpmSetLogCallback(alpmHandle, ae.logCallback())
	ae.handle = alpmHandle
	ae.syncDBsCache = nil

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
	return ae.SyncSatisfier(pkgName) != nil
}

func (ae *AlpmExecutor) IsCorrectVersionInstalled(pkgName, versionRequired string) bool {
	alpmPackage := ae.localDB.Pkg(pkgName)
	if alpmPackage == nil {
		return false
	}

	return alpmPackage.Version() == versionRequired
}

func (ae *AlpmExecutor) SyncSatisfier(pkgName string) alpm.Package {
	dbs := ae.syncDBs()
	if len(dbs) == 0 {
		return nil
	}
	// Use FindDBSatisfier across sync databases
	dbSlice := make([]alpm.Database, len(dbs))
	copy(dbSlice, dbs)
	return ae.handle.FindDBSatisfier(dbSlice, pkgName)
}

func (ae *AlpmExecutor) PackagesFromGroup(groupName string) []alpm.Package {
	pkgs, err := ae.handle.FindGroupPkgs(ae.syncDBs(), groupName)
	if err != nil {
		return nil
	}
	return pkgs
}

func (ae *AlpmExecutor) PackagesFromGroupAndDB(groupName, dbName string) ([]alpm.Package, error) {
	singleDBs, err := ae.handle.SyncDBListByDBName(dbName)
	if err != nil {
		return nil, err
	}
	return ae.handle.FindGroupPkgs(singleDBs, groupName)
}

func (ae *AlpmExecutor) LocalPackages() []alpm.Package {
	localPackages := []alpm.Package{}
	_ = ae.localDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		localPackages = append(localPackages, pkg)
		return nil
	})
	return localPackages
}

// SyncPackages searches SyncDB for packages or returns all packages if no search param is given.
func (ae *AlpmExecutor) SyncPackages(pkgNames ...string) []alpm.Package {
	repoPackages := []alpm.Package{}
	for _, alpmDB := range ae.syncDBs() {
		if len(pkgNames) == 0 {
			_ = alpmDB.PkgCache().ForEach(func(pkg alpm.Package) error {
				repoPackages = append(repoPackages, pkg)
				return nil
			})
			continue
		}
		_ = alpmDB.Search(pkgNames).ForEach(func(pkg alpm.Package) error {
			repoPackages = append(repoPackages, pkg)
			return nil
		})
	}

	return repoPackages
}

func (ae *AlpmExecutor) LocalPackage(pkgName string) alpm.Package {
	pkg := ae.localDB.Pkg(pkgName)
	if pkg == nil {
		return nil
	}

	return pkg
}

func (ae *AlpmExecutor) syncDBs() []alpm.Database {
	if ae.syncDBsCache == nil {
		ae.syncDBsCache = ae.syncDB
	}

	return ae.syncDBsCache
}

func (ae *AlpmExecutor) SyncPackage(pkgName string) alpm.Package {
	for _, db := range ae.syncDBs() {
		if dbPkg := db.Pkg(pkgName); dbPkg != nil {
			return dbPkg
		}
	}

	return nil
}

func (ae *AlpmExecutor) SyncPackageFromDB(pkgName, dbName string) alpm.Package {
	singleDB, err := ae.handle.SyncDBByName(dbName)
	if err != nil {
		return nil
	}

	return singleDB.Pkg(pkgName)
}

func (ae *AlpmExecutor) SatisfierFromDB(pkgName, dbName string) (alpm.Package, error) {
	singleDBs, err := ae.handle.SyncDBListByDBName(dbName)
	if err != nil {
		return nil, err
	}

	foundPkg := ae.handle.FindDBSatisfier(singleDBs, pkgName)
	if foundPkg == nil {
		return nil, nil
	}

	return foundPkg, nil
}

func (ae *AlpmExecutor) PackageDepends(pkg alpm.Package) []alpm.Depend {
	return pkg.Depends()
}

func (ae *AlpmExecutor) PackageOptionalDepends(pkg alpm.Package) []alpm.Depend {
	return pkg.OptionalDepends()
}

func (ae *AlpmExecutor) PackageProvides(pkg alpm.Package) []alpm.Depend {
	return pkg.Provides()
}

func (ae *AlpmExecutor) PackageGroups(pkg alpm.Package) []string {
	return pkg.Groups()
}

// upRepo gathers local packages and checks if they have new versions.
// Output: Upgrade type package list.
func (ae *AlpmExecutor) SyncUpgrades(enableDowngrade bool) (
	map[string]db.SyncUpgrade, error,
) {
	ups := map[string]db.SyncUpgrade{}
	var errReturn error

	localDB, errDB := ae.handle.LocalDB()
	if errDB != nil {
		return ups, errDB
	}

	if err := ae.handle.TransInit(alpm.TransFlagNoLock); err != nil {
		return ups, err
	}

	defer func() {
		errReturn = ae.handle.TransRelease()
	}()

	if err := ae.handle.SyncSysupgrade(enableDowngrade); err != nil {
		return ups, err
	}

	_ = ae.handle.TransGetAdd().ForEach(func(pkg alpm.Package) error {
		localVer := "-"
		reason := alpm.PkgReasonExplicit

		if localPkg := localDB.Pkg(pkg.Name()); localPkg != nil {
			localVer = localPkg.Version()
			reason = localPkg.Reason()
		}

		ups[pkg.Name()] = db.SyncUpgrade{
			Package:      pkg,
			Reason:       reason,
			LocalVersion: localVer,
		}

		return nil
	})

	return ups, errReturn
}

func (ae *AlpmExecutor) BiggestPackages() []alpm.Package {
	return append([]alpm.Package{}, ae.localDB.PkgCache().SortBySize()...)
}

func (ae *AlpmExecutor) LastBuildTime() time.Time {
	var lastTime time.Time

	for _, db := range ae.syncDBs() {
		_ = db.PkgCache().ForEach(func(pkg alpm.Package) error {
			thisTime := pkg.BuildDate()
			if thisTime.After(lastTime) {
				lastTime = thisTime
			}
			return nil
		})
	}

	return lastTime
}

func (ae *AlpmExecutor) Cleanup() {
	if ae.handle != nil {
		if err := ae.handle.Release(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func (ae *AlpmExecutor) Repos() (repos []string) {
	for _, db := range ae.syncDBs() {
		repos = append(repos, db.Name())
	}

	return
}

func alpmSetArchitecture(alpmHandle alpm.Handle, arch []string) error {
	return alpmHandle.SetArchitectures(arch)
}

func (ae *AlpmExecutor) AlpmArchitectures() ([]string, error) {
	architectures, err := ae.handle.Architectures()

	return architectures, err
}

func alpmSetLogCallback(alpmHandle alpm.Handle, cb func(alpm.LogLevel, string)) {
	// dyalpm uses a different callback mechanism - log callback not easily supported
	// due to va_list in libalpm. Skip setting log callback.
	_ = alpmHandle
	_ = cb
}

func alpmSetQuestionCallback(alpmHandle alpm.Handle, cb func(alpm.QuestionAny)) error {
	return alpmHandle.SetQuestionCallbackFunc(func(q alpm.Question) {
		cb(alpm.QuestionAny{Question: q})
	})
}
