package dep

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"
	rpc "github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

type Target struct {
	DB      string
	Name    string
	Mod     string
	Version string
}

func ToTarget(pkg string) Target {
	db, dep := text.SplitDBFromName(pkg)
	name, mod, depVersion := splitDep(dep)

	return Target{
		DB:      db,
		Name:    name,
		Mod:     mod,
		Version: depVersion,
	}
}

func (t Target) DepString() string {
	return t.Name + t.Mod + t.Version
}

func (t Target) String() string {
	if t.DB != "" {
		return t.DB + "/" + t.DepString()
	}

	return t.DepString()
}

type Pool struct {
	Targets  []Target
	Explicit stringset.StringSet
	Repo     map[string]*alpm.Package
	Aur      map[string]*rpc.Pkg
	AurCache map[string]*rpc.Pkg
	Groups   []string
	LocalDB  *alpm.DB
	SyncDB   alpm.DBList
	Warnings *query.AURWarnings
}

func makePool(alpmHandle *alpm.Handle) (*Pool, error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return nil, err
	}
	syncDB, err := alpmHandle.SyncDBs()
	if err != nil {
		return nil, err
	}

	dp := &Pool{
		make([]Target, 0),
		make(stringset.StringSet),
		make(map[string]*alpm.Package),
		make(map[string]*rpc.Pkg),
		make(map[string]*rpc.Pkg),
		make([]string, 0),
		localDB,
		syncDB,
		nil,
	}

	return dp, nil
}

// Includes db/ prefixes and group installs
func (dp *Pool) ResolveTargets(pkgs []string, alpmHandle *alpm.Handle,
	mode settings.TargetMode,
	ignoreProviders, noConfirm, provides bool, rebuild string, splitN int) error {
	// RPC requests are slow
	// Combine as many AUR package requests as possible into a single RPC
	// call
	aurTargets := make(stringset.StringSet)

	pkgs = query.RemoveInvalidTargets(pkgs, mode)

	for _, pkg := range pkgs {
		var err error
		target := ToTarget(pkg)

		// skip targets already satisfied
		// even if the user enters db/pkg and aur/pkg the latter will
		// still get skipped even if it's from a different database to
		// the one specified
		// this is how pacman behaves
		if dp.hasPackage(target.DepString()) {
			continue
		}

		var foundPkg *alpm.Package
		var singleDB *alpm.DB

		// aur/ prefix means we only check the aur
		if target.DB == "aur" || mode == settings.ModeAUR {
			dp.Targets = append(dp.Targets, target)
			aurTargets.Set(target.DepString())
			continue
		}

		// If there'ss a different priefix only look in that repo
		if target.DB != "" {
			singleDB, err = alpmHandle.SyncDBByName(target.DB)
			if err != nil {
				return err
			}
			foundPkg, err = singleDB.PkgCache().FindSatisfier(target.DepString())
			// otherwise find it in any repo
		} else {
			foundPkg, err = dp.SyncDB.FindSatisfier(target.DepString())
		}

		if err == nil {
			dp.Targets = append(dp.Targets, target)
			dp.Explicit.Set(foundPkg.Name())
			dp.ResolveRepoDependency(foundPkg)
			continue
		} else {
			// check for groups
			// currently we don't resolve the packages in a group
			// only check if the group exists
			// would be better to check the groups from singleDB if
			// the user specified a db but there's no easy way to do
			// it without making alpm_lists so don't bother for now
			// db/group is probably a rare use case
			group := dp.SyncDB.FindGroupPkgs(target.Name)
			if !group.Empty() {
				dp.Groups = append(dp.Groups, target.String())
				_ = group.ForEach(func(pkg alpm.Package) error {
					dp.Explicit.Set(pkg.Name())
					return nil
				})
				continue
			}
		}

		// if there was no db prefix check the aur
		if target.DB == "" {
			aurTargets.Set(target.DepString())
		}

		dp.Targets = append(dp.Targets, target)
	}

	if len(aurTargets) > 0 && (mode == settings.ModeAny || mode == settings.ModeAUR) {
		return dp.resolveAURPackages(aurTargets, true, ignoreProviders, noConfirm, provides, rebuild, splitN)
	}

	return nil
}

// Pseudo provides finder.
// Try to find provides by performing a search of the package name
// This effectively performs -Ss on each package
// then runs -Si on each result to cache the information.
//
// For example if you were to -S yay then yay -Ss would give:
// yay-git yay-bin yay realyog pacui pacui-git ruby-yard
// These packages will all be added to the cache in case they are needed later
// Ofcouse only the first three packages provide yay, the rest are just false
// positives.
//
// This method increases dependency resolve time
func (dp *Pool) findProvides(pkgs stringset.StringSet) error {
	var mux sync.Mutex
	var wg sync.WaitGroup

	doSearch := func(pkg string) {
		defer wg.Done()
		var err error
		var results []rpc.Pkg

		// Hack for a bigger search result, if the user wants
		// java-envronment we can search for just java instead and get
		// more hits.
		words := strings.Split(pkg, "-")

		for i := range words {
			results, err = rpc.Search(strings.Join(words[:i+1], "-"))
			if err == nil {
				break
			}
		}

		if err != nil {
			return
		}

		for iR := range results {
			mux.Lock()
			if _, ok := dp.AurCache[results[iR].Name]; !ok {
				pkgs.Set(results[iR].Name)
			}
			mux.Unlock()
		}
	}

	for pkg := range pkgs {
		if dp.LocalDB.Pkg(pkg) != nil {
			continue
		}
		wg.Add(1)
		go doSearch(pkg)
	}

	wg.Wait()

	return nil
}

func (dp *Pool) cacheAURPackages(_pkgs stringset.StringSet, provides bool, splitN int) error {
	pkgs := _pkgs.Copy()
	toQuery := make([]string, 0)

	for pkg := range pkgs {
		if _, ok := dp.AurCache[pkg]; ok {
			pkgs.Remove(pkg)
		}
	}

	if len(pkgs) == 0 {
		return nil
	}

	if provides {
		err := dp.findProvides(pkgs)
		if err != nil {
			return err
		}
	}

	for pkg := range pkgs {
		if _, ok := dp.AurCache[pkg]; !ok {
			name, _, ver := splitDep(pkg)
			if ver != "" {
				toQuery = append(toQuery, name, name+"-"+ver)
			} else {
				toQuery = append(toQuery, name)
			}
		}
	}

	info, err := query.AURInfo(toQuery, dp.Warnings, splitN)
	if err != nil {
		return err
	}

	for _, pkg := range info {
		// Dump everything in cache just in case we need it later
		dp.AurCache[pkg.Name] = pkg
	}

	return nil
}

func (dp *Pool) resolveAURPackages(pkgs stringset.StringSet,
	explicit, ignoreProviders, noConfirm, provides bool,
	rebuild string, splitN int) error {
	newPackages := make(stringset.StringSet)
	newAURPackages := make(stringset.StringSet)

	err := dp.cacheAURPackages(pkgs, provides, splitN)
	if err != nil {
		return err
	}

	if len(pkgs) == 0 {
		return nil
	}

	for name := range pkgs {
		_, ok := dp.Aur[name]
		if ok {
			continue
		}

		pkg := dp.findSatisfierAurCache(name, ignoreProviders, noConfirm, provides)
		if pkg == nil {
			continue
		}

		if explicit {
			dp.Explicit.Set(pkg.Name)
		}
		dp.Aur[pkg.Name] = pkg

		for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
			for _, dep := range deps {
				newPackages.Set(dep)
			}
		}
	}

	for dep := range newPackages {
		if dp.hasSatisfier(dep) {
			continue
		}

		_, isInstalled := dp.LocalDB.PkgCache().FindSatisfier(dep) // has satisfier installed: skip
		hm := settings.HideMenus
		settings.HideMenus = isInstalled == nil
		repoPkg, inRepos := dp.SyncDB.FindSatisfier(dep) // has satisfier in repo: fetch it
		settings.HideMenus = hm
		if isInstalled == nil && (rebuild != "tree" || inRepos == nil) {
			continue
		}

		if inRepos == nil {
			dp.ResolveRepoDependency(repoPkg)
			continue
		}

		// assume it's in the aur
		// ditch the versioning because the RPC can't handle it
		newAURPackages.Set(dep)
	}

	err = dp.resolveAURPackages(newAURPackages, false, ignoreProviders, noConfirm, provides, rebuild, splitN)
	return err
}

func (dp *Pool) ResolveRepoDependency(pkg *alpm.Package) {
	dp.Repo[pkg.Name()] = pkg

	_ = pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		// have satisfier in dep tree: skip
		if dp.hasSatisfier(dep.String()) {
			return
		}

		// has satisfier installed: skip
		_, isInstalled := dp.LocalDB.PkgCache().FindSatisfier(dep.String())
		if isInstalled == nil {
			return
		}

		// has satisfier in repo: fetch it
		repoPkg, inRepos := dp.SyncDB.FindSatisfier(dep.String())
		if inRepos != nil {
			return
		}

		dp.ResolveRepoDependency(repoPkg)

		return nil
	})
}

func GetPool(pkgs []string,
	warnings *query.AURWarnings,
	alpmHandle *alpm.Handle,
	mode settings.TargetMode,
	ignoreProviders, noConfirm, provides bool,
	rebuild string, splitN int) (*Pool, error) {
	dp, err := makePool(alpmHandle)
	if err != nil {
		return nil, err
	}

	dp.Warnings = warnings
	err = dp.ResolveTargets(pkgs, alpmHandle, mode, ignoreProviders, noConfirm, provides, rebuild, splitN)

	return dp, err
}

func (dp *Pool) findSatisfierAur(dep string) *rpc.Pkg {
	for _, pkg := range dp.Aur {
		if satisfiesAur(dep, pkg) {
			return pkg
		}
	}

	return nil
}

// This is mostly used to promote packages from the cache
// to the Install list
// Provide a pacman style provider menu if there's more than one candidate
// This acts slightly differently from Pacman, It will give
// a menu even if a package with a matching name exists. I believe this
// method is better because most of the time you are choosing between
// foo and foo-git.
// Using Pacman's ways trying to install foo would never give you
// a menu.
// TODO: maybe intermix repo providers in the menu
func (dp *Pool) findSatisfierAurCache(dep string, ignoreProviders, noConfirm, provides bool) *rpc.Pkg {
	depName, _, _ := splitDep(dep)
	seen := make(stringset.StringSet)
	providerSlice := makeProviders(depName)

	if dp.LocalDB.Pkg(depName) != nil {
		if pkg, ok := dp.AurCache[dep]; ok && pkgSatisfies(pkg.Name, pkg.Version, dep) {
			return pkg
		}
	}

	if ignoreProviders {
		for _, pkg := range dp.AurCache {
			if pkgSatisfies(pkg.Name, pkg.Version, dep) {
				for _, target := range dp.Targets {
					if target.Name == pkg.Name {
						return pkg
					}
				}
			}
		}
	}

	for _, pkg := range dp.AurCache {
		if seen.Get(pkg.Name) {
			continue
		}

		if pkgSatisfies(pkg.Name, pkg.Version, dep) {
			providerSlice.Pkgs = append(providerSlice.Pkgs, pkg)
			seen.Set(pkg.Name)
			continue
		}

		for _, provide := range pkg.Provides {
			if provideSatisfies(provide, dep) {
				providerSlice.Pkgs = append(providerSlice.Pkgs, pkg)
				seen.Set(pkg.Name)
				continue
			}
		}
	}

	if !provides && providerSlice.Len() >= 1 {
		return providerSlice.Pkgs[0]
	}

	if providerSlice.Len() == 1 {
		return providerSlice.Pkgs[0]
	}

	if providerSlice.Len() > 1 {
		sort.Sort(providerSlice)
		return providerMenu(dep, providerSlice, noConfirm)
	}

	return nil
}

func (dp *Pool) findSatisfierRepo(dep string) *alpm.Package {
	for _, pkg := range dp.Repo {
		if satisfiesRepo(dep, pkg) {
			return pkg
		}
	}

	return nil
}

func (dp *Pool) hasSatisfier(dep string) bool {
	return dp.findSatisfierRepo(dep) != nil || dp.findSatisfierAur(dep) != nil
}

func (dp *Pool) hasPackage(name string) bool {
	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			return true
		}
	}

	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			return true
		}
	}

	for _, pkg := range dp.Groups {
		if pkg == name {
			return true
		}
	}

	return false
}

func providerMenu(dep string, providers providers, noConfirm bool) *rpc.Pkg {
	size := providers.Len()

	str := text.Bold(gotext.Get("There are %d providers available for %s:\n", size, dep))

	size = 1
	str += text.SprintOperationInfo(gotext.Get("Repository AUR"), "\n    ")

	for _, pkg := range providers.Pkgs {
		str += fmt.Sprintf("%d) %s ", size, pkg.Name)
		size++
	}

	text.OperationInfoln(str)

	for {
		fmt.Print(gotext.Get("\nEnter a number (default=1): "))

		if noConfirm {
			fmt.Println("1")
			return providers.Pkgs[0]
		}

		reader := bufio.NewReader(os.Stdin)
		numberBuf, overflow, err := reader.ReadLine()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}

		if overflow {
			text.Errorln(gotext.Get("input too long"))
			continue
		}

		if string(numberBuf) == "" {
			return providers.Pkgs[0]
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			text.Errorln(gotext.Get("invalid number: %s", string(numberBuf)))
			continue
		}

		if num < 1 || num >= size {
			text.Errorln(gotext.Get("invalid value: %d is not between %d and %d", num, 1, size-1))
			continue
		}

		return providers.Pkgs[num-1]
	}

	return nil
}
