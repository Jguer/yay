package dep

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"
	"github.com/mikkeloscar/aur"

	"github.com/Jguer/yay/v10/pkg/db"
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
	dbName, depString := text.SplitDBFromName(pkg)
	name, mod, depVersion := splitDep(depString)

	return Target{
		DB:      dbName,
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
	Targets      []Target
	Explicit     stringset.StringSet
	Repo         map[string]db.IPackage
	Aur          map[string]*query.Pkg
	AurCache     map[string]*query.Pkg
	Groups       []string
	AlpmExecutor db.Executor
	Warnings     *query.AURWarnings
}

func makePool(dbExecutor db.Executor) *Pool {
	dp := &Pool{
		make([]Target, 0),
		make(stringset.StringSet),
		make(map[string]db.IPackage),
		make(map[string]*query.Pkg),
		make(map[string]*query.Pkg),
		make([]string, 0),
		dbExecutor,
		nil,
	}

	return dp
}

// Includes db/ prefixes and group installs
func (dp *Pool) ResolveTargets(pkgs []string,
	mode settings.TargetMode,
	ignoreProviders, noConfirm, provides bool, rebuild string, splitN int, noDeps, noCheckDeps bool) error {
	// RPC requests are slow
	// Combine as many AUR package requests as possible into a single RPC
	// call
	aurTargets := make(stringset.StringSet)

	pkgs = query.RemoveInvalidTargets(pkgs, mode)

	for _, pkg := range pkgs {
		target := ToTarget(pkg)

		// skip targets already satisfied
		// even if the user enters db/pkg and aur/pkg the latter will
		// still get skipped even if it's from a different database to
		// the one specified
		// this is how pacman behaves
		if dp.hasPackage(target.DepString()) {
			continue
		}

		var foundPkg db.IPackage

		// aur/ prefix means we only check the aur
		if target.DB == "aur" || mode == settings.ModeAUR {
			dp.Targets = append(dp.Targets, target)
			aurTargets.Set(target.DepString())
			continue
		}

		// If there's a different prefix only look in that repo
		if target.DB != "" {
			foundPkg = dp.AlpmExecutor.SatisfierFromDB(target.DepString(), target.DB)
		} else {
			// otherwise find it in any repo
			foundPkg = dp.AlpmExecutor.SyncSatisfier(target.DepString())
		}

		if foundPkg != nil {
			dp.Targets = append(dp.Targets, target)
			dp.Explicit.Set(foundPkg.Name())
			dp.ResolveRepoDependency(foundPkg, noDeps)
			continue
		} else {
			// check for groups
			// currently we don't resolve the packages in a group
			// only check if the group exists
			// would be better to check the groups from singleDB if
			// the user specified a db but there's no easy way to do
			// it without making alpm_lists so don't bother for now
			// db/group is probably a rare use case
			groupPackages := dp.AlpmExecutor.PackagesFromGroup(target.Name)
			if len(groupPackages) > 0 {
				dp.Groups = append(dp.Groups, target.String())
				for _, pkg := range groupPackages {
					dp.Explicit.Set(pkg.Name())
				}
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
		return dp.resolveAURPackages(aurTargets, true, ignoreProviders, noConfirm, provides, rebuild, splitN, noDeps, noCheckDeps)
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
		var results []query.Pkg

		// Hack for a bigger search result, if the user wants
		// java-envronment we can search for just java instead and get
		// more hits.
		pkg, _, _ = splitDep(pkg) // openimagedenoise-git > ispc-git #1234
		words := strings.Split(pkg, "-")

		for i := range words {
			results, err = query.Search(strings.Join(words[:i+1], "-"))
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
		if dp.AlpmExecutor.LocalPackage(pkg) != nil {
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

func ComputeCombinedDepList(pkg *aur.Pkg, noDeps, noCheckDeps bool) [][]string {
	combinedDepList := [][]string{pkg.MakeDepends}
	if !noDeps {
		combinedDepList = append(combinedDepList, pkg.Depends)
	}

	if !noCheckDeps {
		combinedDepList = append(combinedDepList, pkg.CheckDepends)
	}

	return combinedDepList
}

func (dp *Pool) resolveAURPackages(pkgs stringset.StringSet,
	explicit, ignoreProviders, noConfirm, provides bool,
	rebuild string, splitN int, noDeps, noCheckDeps bool) error {
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

		combinedDepList := ComputeCombinedDepList(pkg, noDeps, noCheckDeps)
		for _, deps := range combinedDepList {
			for _, dep := range deps {
				newPackages.Set(dep)
			}
		}
	}

	for dep := range newPackages {
		if dp.hasSatisfier(dep) {
			continue
		}

		isInstalled := dp.AlpmExecutor.LocalSatisfierExists(dep)
		hm := settings.HideMenus
		settings.HideMenus = isInstalled
		repoPkg := dp.AlpmExecutor.SyncSatisfier(dep) // has satisfier in repo: fetch it
		settings.HideMenus = hm
		if isInstalled && (rebuild != "tree" || repoPkg != nil) {
			continue
		}

		if repoPkg != nil {
			dp.ResolveRepoDependency(repoPkg, false)
			continue
		}

		// assume it's in the aur
		// ditch the versioning because the RPC can't handle it
		newAURPackages.Set(dep)
	}

	err = dp.resolveAURPackages(newAURPackages, false, ignoreProviders, noConfirm, provides, rebuild, splitN, noDeps, noCheckDeps)
	return err
}

func (dp *Pool) ResolveRepoDependency(pkg db.IPackage, noDeps bool) {
	dp.Repo[pkg.Name()] = pkg
	if noDeps {
		return
	}

	for _, dep := range dp.AlpmExecutor.PackageDepends(pkg) {
		if dp.hasSatisfier(dep.String()) {
			continue
		}

		// has satisfier installed: skip
		if dp.AlpmExecutor.LocalSatisfierExists(dep.String()) {
			continue
		}

		// has satisfier in repo: fetch it
		if repoPkg := dp.AlpmExecutor.SyncSatisfier(dep.String()); repoPkg != nil {
			dp.ResolveRepoDependency(repoPkg, noDeps)
		}
	}
}

func GetPool(pkgs []string,
	warnings *query.AURWarnings,
	dbExecutor db.Executor,
	mode settings.TargetMode,
	ignoreProviders, noConfirm, provides bool,
	rebuild string, splitN int, noDeps bool, noCheckDeps bool) (*Pool, error) {
	dp := makePool(dbExecutor)

	dp.Warnings = warnings
	err := dp.ResolveTargets(pkgs, mode, ignoreProviders, noConfirm, provides, rebuild, splitN, noDeps, noCheckDeps)

	return dp, err
}

func (dp *Pool) findSatisfierAur(dep string) *query.Pkg {
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
func (dp *Pool) findSatisfierAurCache(dep string, ignoreProviders, noConfirm, provides bool) *query.Pkg {
	depName, _, _ := splitDep(dep)
	seen := make(stringset.StringSet)
	providerSlice := makeProviders(depName)

	if dp.AlpmExecutor.LocalPackage(depName) != nil {
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
			if provideSatisfies(provide, dep, pkg.Version) {
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

func (dp *Pool) findSatisfierRepo(dep string) db.IPackage {
	for _, pkg := range dp.Repo {
		if satisfiesRepo(dep, pkg, dp.AlpmExecutor) {
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

func providerMenu(dep string, providers providers, noConfirm bool) *query.Pkg {
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
