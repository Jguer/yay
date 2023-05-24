package ialpm

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	alpm "github.com/Jguer/go-alpm/v2"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
)

type AlpmExecutor struct {
	handle       *alpm.Handle
	localDB      alpm.IDB
	syncDB       alpm.IDBList
	syncDBsCache []alpm.IDB
	conf         *pacmanconf.Config
	log          *text.Logger

	installedRemotePkgNames []string
	installedRemotePkgMap   map[string]alpm.IPackage
	installedSyncPkgNames   []string
}

func NewExecutor(pacmanConf *pacmanconf.Config, logger *text.Logger) (*AlpmExecutor, error) {
	ae := &AlpmExecutor{
		handle:                  nil,
		localDB:                 nil,
		syncDB:                  nil,
		syncDBsCache:            []alpm.IDB{},
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

func configureAlpm(pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle) error {
	for _, repo := range pacmanConf.Repos {
		// TODO: set SigLevel
		alpmDB, err := alpmHandle.RegisterSyncDB(repo.Name, 0)
		if err != nil {
			return err
		}

		alpmDB.SetServers(repo.Servers)
		alpmDB.SetUsage(toUsage(repo.Usage))
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

		_ = qp.Providers(ae.handle).ForEach(func(pkg alpm.IPackage) error {
			size++
			return nil
		})

		str := text.Bold(gotext.Get("There are %d providers available for %s:", size, qp.Dep()))

		size = 1

		var dbName string

		_ = qp.Providers(ae.handle).ForEach(func(pkg alpm.IPackage) error {
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

	alpmSetQuestionCallback(alpmHandle, ae.questionCallback())
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

func (ae *AlpmExecutor) SyncSatisfier(pkgName string) alpm.IPackage {
	foundPkg, err := ae.syncDB.FindSatisfier(pkgName)
	if err != nil {
		return nil
	}

	return foundPkg
}

func (ae *AlpmExecutor) PackagesFromGroup(groupName string) []alpm.IPackage {
	groupPackages := []alpm.IPackage{}
	_ = ae.syncDB.FindGroupPkgs(groupName).ForEach(func(pkg alpm.IPackage) error {
		groupPackages = append(groupPackages, pkg)

		return nil
	})

	return groupPackages
}

func (ae *AlpmExecutor) LocalPackages() []alpm.IPackage {
	localPackages := []alpm.IPackage{}
	_ = ae.localDB.PkgCache().ForEach(func(pkg alpm.IPackage) error {
		localPackages = append(localPackages, pkg)
		return nil
	})

	return localPackages
}

// SyncPackages searches SyncDB for packages or returns all packages if no search param is given.
func (ae *AlpmExecutor) SyncPackages(pkgNames ...string) []alpm.IPackage {
	repoPackages := []alpm.IPackage{}
	_ = ae.syncDB.ForEach(func(alpmDB alpm.IDB) error {
		if len(pkgNames) == 0 {
			_ = alpmDB.PkgCache().ForEach(func(pkg alpm.IPackage) error {
				repoPackages = append(repoPackages, pkg)
				return nil
			})
		} else {
			_ = alpmDB.Search(pkgNames).ForEach(func(pkg alpm.IPackage) error {
				repoPackages = append(repoPackages, pkg)
				return nil
			})
		}
		return nil
	})

	return repoPackages
}

func (ae *AlpmExecutor) LocalPackage(pkgName string) alpm.IPackage {
	pkg := ae.localDB.Pkg(pkgName)
	if pkg == nil {
		return nil
	}

	return pkg
}

func (ae *AlpmExecutor) syncDBs() []alpm.IDB {
	if ae.syncDBsCache == nil {
		ae.syncDBsCache = ae.syncDB.Slice()
	}

	return ae.syncDBsCache
}

func (ae *AlpmExecutor) SyncPackage(pkgName string) alpm.IPackage {
	for _, db := range ae.syncDBs() {
		if dbPkg := db.Pkg(pkgName); dbPkg != nil {
			return dbPkg
		}
	}

	return nil
}

func (ae *AlpmExecutor) SatisfierFromDB(pkgName, dbName string) alpm.IPackage {
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

func (ae *AlpmExecutor) PackageDepends(pkg alpm.IPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Depends().Slice()
}

func (ae *AlpmExecutor) PackageOptionalDepends(pkg alpm.IPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.OptionalDepends().Slice()
}

func (ae *AlpmExecutor) PackageProvides(pkg alpm.IPackage) []alpm.Depend {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Provides().Slice()
}

func (ae *AlpmExecutor) PackageGroups(pkg alpm.IPackage) []string {
	alpmPackage := pkg.(*alpm.Package)
	return alpmPackage.Groups().Slice()
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

	_ = ae.handle.TransGetAdd().ForEach(func(pkg alpm.IPackage) error {
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

func (ae *AlpmExecutor) BiggestPackages() []alpm.IPackage {
	localPackages := []alpm.IPackage{}
	_ = ae.localDB.PkgCache().SortBySize().ForEach(func(pkg alpm.IPackage) error {
		localPackages = append(localPackages, pkg)
		return nil
	})

	return localPackages
}

func (ae *AlpmExecutor) LastBuildTime() time.Time {
	var lastTime time.Time

	_ = ae.syncDB.ForEach(func(db alpm.IDB) error {
		_ = db.PkgCache().ForEach(func(pkg alpm.IPackage) error {
			thisTime := pkg.BuildDate()
			if thisTime.After(lastTime) {
				lastTime = thisTime
			}
			return nil
		})
		return nil
	})

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
	_ = ae.syncDB.ForEach(func(db alpm.IDB) error {
		repos = append(repos, db.Name())
		return nil
	})

	return
}

func alpmSetArchitecture(alpmHandle *alpm.Handle, arch []string) error {
	return alpmHandle.SetArchitectures(arch)
}

func (ae *AlpmExecutor) AlpmArchitectures() ([]string, error) {
	architectures, err := ae.handle.GetArchitectures()

	return architectures.Slice(), err
}

func alpmSetLogCallback(alpmHandle *alpm.Handle, cb func(alpm.LogLevel, string)) {
	alpmHandle.SetLogCallback(func(ctx interface{}, lvl alpm.LogLevel, msg string) {
		cbo := ctx.(func(alpm.LogLevel, string))
		cbo(lvl, msg)
	}, cb)
}

func alpmSetQuestionCallback(alpmHandle *alpm.Handle, cb func(alpm.QuestionAny)) {
	alpmHandle.SetQuestionCallback(func(ctx interface{}, q alpm.QuestionAny) {
		cbo := ctx.(func(alpm.QuestionAny))
		cbo(q)
	}, cb)
}
