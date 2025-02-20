package dep

import (
	"context"
	"fmt"
	"strconv"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/intrange"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

type InstallInfo struct {
	Source       Source
	Reason       Reason
	Version      string
	LocalVersion string
	SrcinfoPath  *string
	AURBase      *string
	SyncDBName   *string

	IsGroup bool
	Upgrade bool
	Devel   bool
}

func (i *InstallInfo) String() string {
	return fmt.Sprintf("InstallInfo{Source: %v, Reason: %v}", i.Source, i.Reason)
}

type (
	Reason int
	Source int
)

func (r Reason) String() string {
	return ReasonNames[r]
}

func (s Source) String() string {
	return SourceNames[s]
}

const (
	Explicit Reason = iota // 0
	Dep                    // 1
	MakeDep                // 2
	CheckDep               // 3
)

var ReasonNames = map[Reason]string{
	Explicit: gotext.Get("Explicit"),
	Dep:      gotext.Get("Dependency"),
	MakeDep:  gotext.Get("Make Dependency"),
	CheckDep: gotext.Get("Check Dependency"),
}

const (
	AUR Source = iota
	Sync
	Local
	SrcInfo
	Missing
)

var SourceNames = map[Source]string{
	AUR:     gotext.Get("AUR"),
	Sync:    gotext.Get("Sync"),
	Local:   gotext.Get("Local"),
	SrcInfo: gotext.Get("SRCINFO"),
	Missing: gotext.Get("Missing"),
}

var bgColorMap = map[Source]string{
	AUR:     "lightblue",
	Sync:    "lemonchiffon",
	Local:   "darkolivegreen1",
	Missing: "tomato",
}

var colorMap = map[Reason]string{
	Explicit: "black",
	Dep:      "deeppink",
	MakeDep:  "navyblue",
	CheckDep: "forestgreen",
}

type Grapher struct {
	logger        *text.Logger
	providerCache map[string][]aur.Pkg

	dbExecutor  db.Executor
	aurClient   aurc.QueryClient
	fullGraph   bool // If true, the graph will include all dependencies including already installed ones or repo
	noConfirm   bool // If true, the graph will not prompt for confirmation
	noDeps      bool // If true, the graph will not include dependencies
	noCheckDeps bool // If true, the graph will not include check dependencies
	needed      bool // If true, the graph will only include packages that are not installed
}

func NewGrapher(dbExecutor db.Executor, aurCache aurc.QueryClient,
	fullGraph, noConfirm, noDeps, noCheckDeps, needed bool,
	logger *text.Logger,
) *Grapher {
	return &Grapher{
		dbExecutor:    dbExecutor,
		aurClient:     aurCache,
		fullGraph:     fullGraph,
		noConfirm:     noConfirm,
		noDeps:        noDeps,
		noCheckDeps:   noCheckDeps,
		needed:        needed,
		providerCache: make(map[string][]aurc.Pkg, 5),
		logger:        logger,
	}
}

func NewGraph() *topo.Graph[string, *InstallInfo] {
	return topo.New[string, *InstallInfo]()
}

func (g *Grapher) GraphFromTargets(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo], targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	aurTargets := make([]string, 0, len(targets))

	for _, targetString := range targets {
		target := ToTarget(targetString)

		switch target.DB {
		case "": // unspecified db
			if pkg := g.dbExecutor.SyncSatisfier(target.Name); pkg != nil {
				g.GraphSyncPkg(ctx, graph, pkg, nil)

				continue
			}

			groupPackages := g.dbExecutor.PackagesFromGroup(target.Name)
			if len(groupPackages) > 0 {
				dbName := groupPackages[0].DB().Name()
				g.GraphSyncGroup(ctx, graph, target.Name, dbName)

				continue
			}

			fallthrough
		case "aur":
			aurTargets = append(aurTargets, target.Name)
		default:
			pkg, err := g.dbExecutor.SatisfierFromDB(target.Name, target.DB)
			if err != nil {
				return nil, err
			}
			if pkg != nil {
				g.GraphSyncPkg(ctx, graph, pkg, nil)
				continue
			}

			groupPackages, err := g.dbExecutor.PackagesFromGroupAndDB(target.Name, target.DB)
			if err != nil {
				return nil, err
			}
			if len(groupPackages) > 0 {
				g.GraphSyncGroup(ctx, graph, target.Name, target.DB)

				continue
			}

			g.logger.Errorln(gotext.Get("No package found for"), " ", target)
		}
	}

	var errA error
	graph, errA = g.GraphFromAUR(ctx, graph, aurTargets)
	if errA != nil {
		return nil, errA
	}

	return graph, nil
}

func (g *Grapher) pickSrcInfoPkgs(pkgs []*aurc.Pkg) ([]*aurc.Pkg, error) {
	final := make([]*aurc.Pkg, 0, len(pkgs))
	for i := range pkgs {
		g.logger.Println(text.Magenta(strconv.Itoa(i+1)+" ") + text.Bold(pkgs[i].Name) +
			" " + text.Cyan(pkgs[i].Version))
		g.logger.Println("    " + pkgs[i].Description)
	}
	g.logger.Infoln(gotext.Get("Packages to exclude") + " (eg: \"1 2 3\", \"1-3\", \"^4\"):")

	numberBuf, err := g.logger.GetInput("", g.noConfirm)
	if err != nil {
		return nil, err
	}

	include, exclude, _, otherExclude := intrange.ParseNumberMenu(numberBuf)
	isInclude := len(exclude) == 0 && otherExclude.Cardinality() == 0

	for i := 1; i <= len(pkgs); i++ {
		target := i - 1

		if isInclude && !include.Get(i) {
			final = append(final, pkgs[target])
		}

		if !isInclude && (exclude.Get(i)) {
			final = append(final, pkgs[target])
		}
	}

	return final, nil
}

func (g *Grapher) addAurPkgProvides(pkg *aurc.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	for i := range pkg.Provides {
		depName, mod, version := splitDep(pkg.Provides[i])
		g.logger.Debugln(pkg.String() + " provides: " + depName)
		graph.Provides(depName, &alpm.Depend{
			Name:    depName,
			Version: version,
			Mod:     aurDepModToAlpmDep(mod),
		}, pkg.Name)
	}
}

func (g *Grapher) GraphFromSrcInfos(ctx context.Context, graph *topo.Graph[string, *InstallInfo],
	srcInfos map[string]*gosrc.Srcinfo,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	aurPkgsAdded := []*aurc.Pkg{}
	for pkgBuildDir, pkgbuild := range srcInfos {
		pkgBuildDir := pkgBuildDir

		aurPkgs, err := makeAURPKGFromSrcinfo(g.dbExecutor, pkgbuild)
		if err != nil {
			return nil, err
		}

		if len(aurPkgs) > 1 {
			var errPick error
			aurPkgs, errPick = g.pickSrcInfoPkgs(aurPkgs)
			if errPick != nil {
				return nil, errPick
			}
		}

		for _, pkg := range aurPkgs {
			pkg := pkg

			reason := Explicit
			if pkg := g.dbExecutor.LocalPackage(pkg.Name); pkg != nil {
				reason = Reason(pkg.Reason())
			}

			graph.AddNode(pkg.Name)

			g.addAurPkgProvides(pkg, graph)

			g.ValidateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
				Color:      colorMap[reason],
				Background: bgColorMap[AUR],
				Value: &InstallInfo{
					Source:      SrcInfo,
					Reason:      reason,
					SrcinfoPath: &pkgBuildDir,
					AURBase:     &pkg.PackageBase,
					Version:     pkg.Version,
				},
			})
		}

		aurPkgsAdded = append(aurPkgsAdded, aurPkgs...)
	}

	g.AddDepsForPkgs(ctx, aurPkgsAdded, graph)

	return graph, nil
}

func (g *Grapher) AddDepsForPkgs(ctx context.Context, pkgs []*aur.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	for _, pkg := range pkgs {
		g.addDepNodes(ctx, pkg, graph)
	}
}

func (g *Grapher) addDepNodes(ctx context.Context, pkg *aur.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	if len(pkg.MakeDepends) > 0 {
		g.addNodes(ctx, graph, pkg.Name, pkg.MakeDepends, MakeDep)
	}

	if !g.noDeps && len(pkg.Depends) > 0 {
		g.addNodes(ctx, graph, pkg.Name, pkg.Depends, Dep)
	}

	if !g.noCheckDeps && !g.noDeps && len(pkg.CheckDepends) > 0 {
		g.addNodes(ctx, graph, pkg.Name, pkg.CheckDepends, CheckDep)
	}
}

func (g *Grapher) GraphSyncPkg(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	pkg alpm.IPackage, upgradeInfo *db.SyncUpgrade,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(pkg.Name())
	_ = pkg.Provides().ForEach(func(p *alpm.Depend) error {
		g.logger.Debugln(pkg.Name() + " provides: " + p.String())
		graph.Provides(p.Name, p, pkg.Name())
		return nil
	})

	dbName := pkg.DB().Name()
	info := &InstallInfo{
		Source:     Sync,
		Reason:     Explicit,
		Version:    pkg.Version(),
		SyncDBName: &dbName,
	}

	if upgradeInfo == nil {
		if localPkg := g.dbExecutor.LocalPackage(pkg.Name()); localPkg != nil {
			info.Reason = Reason(localPkg.Reason())
		}
	} else {
		info.Upgrade = true
		info.Reason = Reason(upgradeInfo.Reason)
		info.LocalVersion = upgradeInfo.LocalVersion
	}

	g.ValidateAndSetNodeInfo(graph, pkg.Name(), &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[info.Reason],
		Background: bgColorMap[info.Source],
		Value:      info,
	})

	return graph
}

func (g *Grapher) GraphSyncGroup(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	groupName, dbName string,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(groupName)

	g.ValidateAndSetNodeInfo(graph, groupName, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[Explicit],
		Background: bgColorMap[Sync],
		Value: &InstallInfo{
			Source:     Sync,
			Reason:     Explicit,
			Version:    "",
			SyncDBName: &dbName,
			IsGroup:    true,
		},
	})

	return graph
}

func (g *Grapher) GraphAURTarget(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	pkg *aurc.Pkg, instalInfo *InstallInfo,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(pkg.Name)

	g.addAurPkgProvides(pkg, graph)

	g.ValidateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[instalInfo.Reason],
		Background: bgColorMap[AUR],
		Value:      instalInfo,
	})

	return graph
}

func (g *Grapher) GraphFromAUR(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	if len(targets) == 0 {
		return graph, nil
	}

	aurPkgs, errCache := g.aurClient.Get(ctx, &aurc.Query{By: aurc.Name, Needles: targets})
	if errCache != nil {
		g.logger.Errorln(errCache)
	}

	for i := range aurPkgs {
		pkg := &aurPkgs[i]
		if _, ok := g.providerCache[pkg.Name]; !ok {
			g.providerCache[pkg.Name] = []aurc.Pkg{*pkg}
		}
	}

	aurPkgsAdded := []*aurc.Pkg{}

	for _, target := range targets {
		if cachedProvidePkg, ok := g.providerCache[target]; ok {
			aurPkgs = cachedProvidePkg
		} else {
			var errA error
			aurPkgs, errA = g.aurClient.Get(ctx, &aurc.Query{By: aurc.Provides, Needles: []string{target}, Contains: true})
			if errA != nil {
				g.logger.Errorln(gotext.Get("Failed to find AUR package for"), " ", target, ":", errA)
			}
		}

		if len(aurPkgs) == 0 {
			g.logger.Errorln(gotext.Get("No AUR package found for"), " ", target)

			continue
		}

		aurPkg := &aurPkgs[0]
		if len(aurPkgs) > 1 {
			chosen := g.provideMenu(target, aurPkgs)
			aurPkg = chosen
			g.providerCache[target] = []aurc.Pkg{*aurPkg}
		}

		reason := Explicit
		if pkg := g.dbExecutor.LocalPackage(aurPkg.Name); pkg != nil {
			reason = Reason(pkg.Reason())

			if g.needed {
				if db.VerCmp(pkg.Version(), aurPkg.Version) >= 0 {
					g.logger.Warnln(gotext.Get("%s is up to date -- skipping", text.Cyan(pkg.Name()+"-"+pkg.Version())))
					continue
				}
			}
		}

		graph = g.GraphAURTarget(ctx, graph, aurPkg, &InstallInfo{
			AURBase: &aurPkg.PackageBase,
			Reason:  reason,
			Source:  AUR,
			Version: aurPkg.Version,
		})
		aurPkgsAdded = append(aurPkgsAdded, aurPkg)
	}

	g.AddDepsForPkgs(ctx, aurPkgsAdded, graph)

	return graph, nil
}

// Removes found deps from the deps mapset and returns the found deps.
func (g *Grapher) findDepsFromAUR(ctx context.Context,
	deps mapset.Set[string],
) []aurc.Pkg {
	pkgsToAdd := make([]aurc.Pkg, 0, deps.Cardinality())
	if deps.Cardinality() == 0 {
		return []aurc.Pkg{}
	}

	missingNeedles := make([]string, 0, deps.Cardinality())
	for _, depString := range deps.ToSlice() {
		if _, ok := g.providerCache[depString]; !ok {
			depName, _, _ := splitDep(depString)
			missingNeedles = append(missingNeedles, depName)
		}
	}

	if len(missingNeedles) != 0 {
		g.logger.Debugln("deps to find", missingNeedles)
		// provider search is more demanding than a simple search
		// try to find name match if possible and then try to find provides.
		aurPkgs, errCache := g.aurClient.Get(ctx, &aurc.Query{
			By: aurc.Name, Needles: missingNeedles, Contains: false,
		})
		if errCache != nil {
			g.logger.Errorln(errCache)
		}

		for i := range aurPkgs {
			pkg := &aurPkgs[i]
			if deps.Contains(pkg.Name) {
				g.providerCache[pkg.Name] = append(g.providerCache[pkg.Name], *pkg)
			}

			for _, val := range pkg.Provides {
				if val == pkg.Name {
					continue
				}
				if deps.Contains(val) {
					g.providerCache[val] = append(g.providerCache[val], *pkg)
				}
			}
		}
	}

	for _, depString := range deps.ToSlice() {
		var aurPkgs []aurc.Pkg
		depName, _, _ := splitDep(depString)

		if cachedProvidePkg, ok := g.providerCache[depString]; ok {
			aurPkgs = cachedProvidePkg
		} else {
			var errA error
			aurPkgs, errA = g.aurClient.Get(ctx, &aurc.Query{By: aurc.Provides, Needles: []string{depName}, Contains: true})
			if errA != nil {
				g.logger.Errorln(gotext.Get("Failed to find AUR package for"), depString, ":", errA)
			}
		}

		// remove packages that don't satisfy the dependency
		satisfyingPkgs := make([]aurc.Pkg, 0, len(aurPkgs))
		for i := range aurPkgs {
			if satisfiesAur(depString, &aurPkgs[i]) {
				satisfyingPkgs = append(satisfyingPkgs, aurPkgs[i])
			}
		}
		aurPkgs = satisfyingPkgs

		if len(aurPkgs) == 0 {
			g.logger.Errorln(gotext.Get("No AUR package found for"), " ", depString)

			continue
		}

		pkg := aurPkgs[0]
		if len(aurPkgs) > 1 {
			chosen := g.provideMenu(depString, aurPkgs)
			pkg = *chosen
		}

		g.providerCache[depString] = []aurc.Pkg{pkg}
		deps.Remove(depString)
		pkgsToAdd = append(pkgsToAdd, pkg)
	}

	return pkgsToAdd
}

func (g *Grapher) ValidateAndSetNodeInfo(graph *topo.Graph[string, *InstallInfo],
	node string, nodeInfo *topo.NodeInfo[*InstallInfo],
) {
	info := graph.GetNodeInfo(node)
	if info != nil && info.Value != nil {
		if info.Value.Reason < nodeInfo.Value.Reason {
			return // refuse to downgrade reason
		}

		if info.Value.Upgrade {
			return // refuse to overwrite an upgrade
		}
	}

	graph.SetNodeInfo(node, nodeInfo)
}

func (g *Grapher) addNodes(
	ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	parentPkgName string,
	deps []string,
	depType Reason,
) {
	targetsToFind := mapset.NewThreadUnsafeSet(deps...)
	// Check if in graph already
	for _, depString := range targetsToFind.ToSlice() {
		depName, _, _ := splitDep(depString)
		if !graph.Exists(depName) && !graph.ProvidesExists(depName) {
			continue
		}

		if graph.Exists(depName) {
			if err := graph.DependOn(depName, parentPkgName); err != nil {
				g.logger.Warnln(depString, parentPkgName, err)
			}

			targetsToFind.Remove(depString)
		}

		if p := graph.GetProviderNode(depName); p != nil {
			if provideSatisfies(p.String(), depString, p.Version) {
				if err := graph.DependOn(p.Provider, parentPkgName); err != nil {
					g.logger.Warnln(p.Provider, parentPkgName, err)
				}

				targetsToFind.Remove(depString)
			}
		}
	}

	// Check installed
	for _, depString := range targetsToFind.ToSlice() {
		depName, _, _ := splitDep(depString)
		if !g.dbExecutor.LocalSatisfierExists(depString) {
			continue
		}

		if g.fullGraph {
			g.ValidateAndSetNodeInfo(
				graph,
				depName,
				&topo.NodeInfo[*InstallInfo]{Color: colorMap[depType], Background: bgColorMap[Local]})

			if err := graph.DependOn(depName, parentPkgName); err != nil {
				g.logger.Warnln(depName, parentPkgName, err)
			}
		}

		targetsToFind.Remove(depString)
	}

	// Check Sync
	for _, depString := range targetsToFind.ToSlice() {
		alpmPkg := g.dbExecutor.SyncSatisfier(depString)
		if alpmPkg == nil {
			continue
		}

		if err := graph.DependOn(alpmPkg.Name(), parentPkgName); err != nil {
			g.logger.Warnln("repo dep warn:", depString, parentPkgName, err)
		}

		dbName := alpmPkg.DB().Name()
		g.ValidateAndSetNodeInfo(
			graph,
			alpmPkg.Name(),
			&topo.NodeInfo[*InstallInfo]{
				Color:      colorMap[depType],
				Background: bgColorMap[Sync],
				Value: &InstallInfo{
					Source:     Sync,
					Reason:     depType,
					Version:    alpmPkg.Version(),
					SyncDBName: &dbName,
				},
			})

		if newDeps := alpmPkg.Depends().Slice(); len(newDeps) != 0 && g.fullGraph {
			newDepsSlice := make([]string, 0, len(newDeps))
			for _, newDep := range newDeps {
				newDepsSlice = append(newDepsSlice, newDep.Name)
			}

			g.addNodes(ctx, graph, alpmPkg.Name(), newDepsSlice, Dep)
		}

		targetsToFind.Remove(depString)
	}

	// Check AUR
	pkgsToAdd := g.findDepsFromAUR(ctx, targetsToFind)
	for i := range pkgsToAdd {
		aurPkg := &pkgsToAdd[i]
		if err := graph.DependOn(aurPkg.Name, parentPkgName); err != nil {
			g.logger.Warnln("aur dep warn:", aurPkg.Name, parentPkgName, err)
		}

		graph.SetNodeInfo(
			aurPkg.Name,
			&topo.NodeInfo[*InstallInfo]{
				Color:      colorMap[depType],
				Background: bgColorMap[AUR],
				Value: &InstallInfo{
					Source:  AUR,
					Reason:  depType,
					AURBase: &aurPkg.PackageBase,
					Version: aurPkg.Version,
				},
			})

		g.addDepNodes(ctx, aurPkg, graph)
	}

	// Add missing to graph
	for _, depString := range targetsToFind.ToSlice() {
		depName, mod, ver := splitDep(depString)
		// no dep found. add as missing
		if err := graph.DependOn(depName, parentPkgName); err != nil {
			g.logger.Warnln("missing dep warn:", depString, parentPkgName, err)
		}
		graph.SetNodeInfo(depName, &topo.NodeInfo[*InstallInfo]{
			Color:      colorMap[depType],
			Background: bgColorMap[Missing],
			Value: &InstallInfo{
				Source:  Missing,
				Reason:  depType,
				Version: fmt.Sprintf("%s%s", mod, ver),
			},
		})
	}
}

func (g *Grapher) provideMenu(dep string, options []aur.Pkg) *aur.Pkg {
	size := len(options)
	if size == 1 {
		return &options[0]
	}

	str := text.Bold(gotext.Get("There are %[1]d providers available for %[2]s:", size, dep))
	str += "\n"

	size = 1
	str += g.logger.SprintOperationInfo(gotext.Get("Repository AUR"), "\n    ")

	for i := range options {
		str += fmt.Sprintf("%d) %s ", size, options[i].Name)
		size++
	}

	g.logger.OperationInfoln(str)

	for {
		g.logger.Println(gotext.Get("\nEnter a number (default=1): "))

		if g.noConfirm {
			g.logger.Println("1")

			return &options[0]
		}

		numberBuf, err := g.logger.GetInput("", false)
		if err != nil {
			g.logger.Errorln(err)

			return &options[0]
		}

		if numberBuf == "" {
			return &options[0]
		}

		num, err := strconv.Atoi(numberBuf)
		if err != nil {
			g.logger.Errorln(gotext.Get("invalid number: %s", numberBuf))

			continue
		}

		if num < 1 || num >= size {
			g.logger.Errorln(gotext.Get("invalid value: %d is not between %d and %d",
				num, 1, size-1))

			continue
		}

		return &options[num-1]
	}
}

func makeAURPKGFromSrcinfo(dbExecutor db.Executor, srcInfo *gosrc.Srcinfo) ([]*aur.Pkg, error) {
	pkgs := make([]*aur.Pkg, 0, 1)

	alpmArch, err := dbExecutor.AlpmArchitectures()
	if err != nil {
		return nil, err
	}

	alpmArch = append(alpmArch, "") // srcinfo assumes no value as ""

	getDesc := func(pkg *gosrc.Package) string {
		if pkg.Pkgdesc != "" {
			return pkg.Pkgdesc
		}

		return srcInfo.Pkgdesc
	}

	for i := range srcInfo.Packages {
		pkg := &srcInfo.Packages[i]

		pkgs = append(pkgs, &aur.Pkg{
			ID:            0,
			Name:          pkg.Pkgname,
			PackageBaseID: 0,
			PackageBase:   srcInfo.Pkgbase,
			Version:       srcInfo.Version(),
			Description:   getDesc(pkg),
			URL:           pkg.URL,
			Depends: append(archStringToString(alpmArch, pkg.Depends),
				archStringToString(alpmArch, srcInfo.Package.Depends)...),
			MakeDepends:  archStringToString(alpmArch, srcInfo.PackageBase.MakeDepends),
			CheckDepends: archStringToString(alpmArch, srcInfo.PackageBase.CheckDepends),
			Conflicts: append(archStringToString(alpmArch, pkg.Conflicts),
				archStringToString(alpmArch, srcInfo.Package.Conflicts)...),
			Provides: append(archStringToString(alpmArch, pkg.Provides),
				archStringToString(alpmArch, srcInfo.Package.Provides)...),
			Replaces: append(archStringToString(alpmArch, pkg.Replaces),
				archStringToString(alpmArch, srcInfo.Package.Replaces)...),
			OptDepends: append(archStringToString(alpmArch, pkg.OptDepends),
				archStringToString(alpmArch, srcInfo.Package.OptDepends)...),
			Groups:   pkg.Groups,
			License:  pkg.License,
			Keywords: []string{},
		})
	}

	return pkgs, nil
}

func archStringToString(alpmArches []string, archString []gosrc.ArchString) []string {
	pkgs := make([]string, 0, len(archString))

	for _, arch := range archString {
		if db.ArchIsSupported(alpmArches, arch.Arch) {
			pkgs = append(pkgs, arch.Value)
		}
	}

	return pkgs
}

func aurDepModToAlpmDep(mod string) alpm.DepMod {
	switch mod {
	case "=":
		return alpm.DepModEq
	case ">=":
		return alpm.DepModGE
	case "<=":
		return alpm.DepModLE
	case ">":
		return alpm.DepModGT
	case "<":
		return alpm.DepModLT
	}
	return alpm.DepModAny
}
