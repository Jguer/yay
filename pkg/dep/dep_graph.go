package dep

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/intrange"
	aur "github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
)

type InstallInfo struct {
	Source       Source
	Reason       Reason
	Version      string
	LocalVersion string
	SrcinfoPath  *string
	AURBase      *string
	SyncDBName   *string

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
	dbExecutor  db.Executor
	aurCache    aurc.QueryClient
	fullGraph   bool // If true, the graph will include all dependencies including already installed ones or repo
	noConfirm   bool
	noDeps      bool // If true, the graph will not include dependencies
	noCheckDeps bool // If true, the graph will not include dependencies

	logger        *text.Logger
	providerCache map[string][]aur.Pkg
}

func NewGrapher(dbExecutor db.Executor, aurCache aurc.QueryClient,
	fullGraph, noConfirm bool, noDeps bool, noCheckDeps bool,
	logger *text.Logger,
) *Grapher {
	return &Grapher{
		dbExecutor:    dbExecutor,
		aurCache:      aurCache,
		fullGraph:     fullGraph,
		noConfirm:     noConfirm,
		noDeps:        noDeps,
		noCheckDeps:   noCheckDeps,
		providerCache: make(map[string][]aurc.Pkg, 5),
		logger:        logger,
	}
}

func (g *Grapher) GraphFromTargets(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo], targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

	for _, targetString := range targets {
		var (
			err    error
			target = ToTarget(targetString)
		)

		switch target.DB {
		case "": // unspecified db
			if pkg := g.dbExecutor.SyncPackage(target.Name); pkg != nil {
				dbName := pkg.DB().Name()
				graph.AddNode(pkg.Name())
				g.ValidateAndSetNodeInfo(graph, pkg.Name(), &topo.NodeInfo[*InstallInfo]{
					Color:      colorMap[Explicit],
					Background: bgColorMap[Sync],
					Value: &InstallInfo{
						Source:     Sync,
						Reason:     Explicit,
						Version:    pkg.Version(),
						SyncDBName: &dbName,
					},
				})

				g.GraphSyncPkg(ctx, graph, pkg, &InstallInfo{
					Source:     Sync,
					Reason:     Explicit,
					Version:    pkg.Version(),
					SyncDBName: &dbName,
				})

				continue
			}

			groupPackages := g.dbExecutor.PackagesFromGroup(target.Name)
			if len(groupPackages) > 0 {
				dbName := groupPackages[0].DB().Name()
				graph.AddNode(target.Name)
				g.ValidateAndSetNodeInfo(graph, target.Name, &topo.NodeInfo[*InstallInfo]{
					Color:      colorMap[Explicit],
					Background: bgColorMap[Sync],
					Value: &InstallInfo{
						Source:     Sync,
						Reason:     Explicit,
						Version:    "",
						SyncDBName: &dbName,
					},
				})

				continue
			}

			fallthrough
		case "aur":
			graph, err = g.GraphFromAURCache(ctx, graph, []string{target.Name})
		default:
			graph.AddNode(target.Name)
			g.ValidateAndSetNodeInfo(graph, target.Name, &topo.NodeInfo[*InstallInfo]{
				Color:      colorMap[Explicit],
				Background: bgColorMap[Sync],
				Value: &InstallInfo{
					Source:     Sync,
					Reason:     Explicit,
					Version:    target.Version,
					SyncDBName: &target.DB,
				},
			})
		}

		if err != nil {
			return nil, err
		}
	}

	return graph, nil
}

func (g *Grapher) pickSrcInfoPkgs(pkgs []aurc.Pkg) ([]aurc.Pkg, error) {
	final := make([]aurc.Pkg, 0, len(pkgs))
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
	isInclude := len(exclude) == 0 && len(otherExclude) == 0

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

func (g *Grapher) GraphFromSrcInfo(ctx context.Context, graph *topo.Graph[string, *InstallInfo], pkgBuildDir string,
	pkgbuild *gosrc.Srcinfo,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

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

	for i := range aurPkgs {
		pkg := &aurPkgs[i]

		graph.AddNode(pkg.Name)
		g.ValidateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
			Color:      colorMap[Explicit],
			Background: bgColorMap[AUR],
			Value: &InstallInfo{
				Source:      SrcInfo,
				Reason:      Explicit,
				SrcinfoPath: &pkgBuildDir,
				AURBase:     &pkg.PackageBase,
				Version:     pkg.Version,
			},
		})

		g.addDepNodes(ctx, pkg, graph)
	}

	return graph, nil
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
	pkg alpm.IPackage, instalInfo *InstallInfo,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

	graph.AddNode(pkg.Name())
	g.ValidateAndSetNodeInfo(graph, pkg.Name(), &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[Explicit],
		Background: bgColorMap[Sync],
		Value:      instalInfo,
	})

	return graph
}

func (g *Grapher) GraphAURTarget(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	pkg *aurc.Pkg, instalInfo *InstallInfo,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

	graph.AddNode(pkg.Name)
	g.ValidateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[Explicit],
		Background: bgColorMap[AUR],
		Value:      instalInfo,
	})

	g.addDepNodes(ctx, pkg, graph)

	return graph
}

func (g *Grapher) GraphFromAURCache(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

	for _, target := range targets {
		aurPkgs, _ := g.aurCache.Get(ctx, &aurc.Query{By: aurc.Name, Needles: []string{target}})
		if len(aurPkgs) == 0 {
			g.logger.Errorln("No AUR package found for", target)

			continue
		}

		pkg := g.provideMenu(target, aurPkgs)

		graph = g.GraphAURTarget(ctx, graph, pkg, &InstallInfo{
			AURBase: &pkg.PackageBase,
			Reason:  Explicit,
			Source:  AUR,
			Version: pkg.Version,
		})
	}

	return graph, nil
}

func (g *Grapher) ValidateAndSetNodeInfo(graph *topo.Graph[string, *InstallInfo],
	node string, nodeInfo *topo.NodeInfo[*InstallInfo],
) {
	info := graph.GetNodeInfo(node)
	if info != nil && info.Value != nil {
		if info.Value.Reason < nodeInfo.Value.Reason {
			return // refuse to downgrade reason from explicit to dep
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
	for _, depString := range deps {
		depName, mod, ver := splitDep(depString)

		if g.dbExecutor.LocalSatisfierExists(depString) {
			if g.fullGraph {
				g.ValidateAndSetNodeInfo(
					graph,
					depName,
					&topo.NodeInfo[*InstallInfo]{Color: colorMap[depType], Background: bgColorMap[Local]})

				if err := graph.DependOn(depName, parentPkgName); err != nil {
					g.logger.Warnln(depName, parentPkgName, err)
				}
			}

			continue
		}

		if graph.Exists(depName) {
			if err := graph.DependOn(depName, parentPkgName); err != nil {
				g.logger.Warnln(depName, parentPkgName, err)
			}

			continue
		}

		// Check ALPM
		if alpmPkg := g.dbExecutor.SyncSatisfier(depString); alpmPkg != nil {
			if err := graph.DependOn(alpmPkg.Name(), parentPkgName); err != nil {
				g.logger.Warnln("repo dep warn:", depName, parentPkgName, err)
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

			continue
		}

		var aurPkgs []aur.Pkg
		if cachedProvidePkg, ok := g.providerCache[depName]; ok {
			aurPkgs = cachedProvidePkg
		} else {
			var errMeta error
			aurPkgs, errMeta = g.aurCache.Get(ctx,
				&aurc.Query{
					Needles:  []string{depName},
					By:       aurc.None,
					Contains: false,
				})
			if errMeta != nil {
				g.logger.Warnln("AUR cache error:", errMeta)
			}
		}

		if len(aurPkgs) != 0 { // Check AUR
			pkg := aurPkgs[0]
			if len(aurPkgs) > 1 {
				chosen := g.provideMenu(depName, aurPkgs)
				pkg = *chosen
				g.providerCache[depName] = []aurc.Pkg{pkg}
			}

			if err := graph.DependOn(pkg.Name, parentPkgName); err != nil {
				g.logger.Warnln("aur dep warn:", pkg.Name, parentPkgName, err)
			}

			graph.SetNodeInfo(
				pkg.Name,
				&topo.NodeInfo[*InstallInfo]{
					Color:      colorMap[depType],
					Background: bgColorMap[AUR],
					Value: &InstallInfo{
						Source:  AUR,
						Reason:  depType,
						AURBase: &pkg.PackageBase,
						Version: pkg.Version,
					},
				})
			g.addDepNodes(ctx, &pkg, graph)

			continue
		}

		// no dep found. add as missing
		graph.AddNode(depName)
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

	str := text.Bold(gotext.Get("There are %d providers available for %s:", size, dep))
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

			break
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

	return nil
}

func makeAURPKGFromSrcinfo(dbExecutor db.Executor, srcInfo *gosrc.Srcinfo) ([]aur.Pkg, error) {
	pkgs := make([]aur.Pkg, 0, 1)

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

		pkgs = append(pkgs, aur.Pkg{
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
			OptDepends: []string{},
			Groups:     pkg.Groups,
			License:    pkg.License,
			Keywords:   []string{},
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
