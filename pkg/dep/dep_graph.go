package dep

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/Jguer/yay/v11/pkg/db"
	aur "github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"

	aurc "github.com/Jguer/aur"
	"github.com/Jguer/aur/metadata"
	alpm "github.com/Jguer/go-alpm/v2"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
)

type InstallInfo struct {
	Source      Source
	Reason      Reason
	Version     string
	SrcinfoPath *string
	AURBase     *string
	SyncDBName  *string
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

type AURCache interface {
	Get(ctx context.Context, query *metadata.AURQuery) ([]aurc.Pkg, error)
}

type Grapher struct {
	dbExecutor  db.Executor
	aurCache    AURCache
	fullGraph   bool // If true, the graph will include all dependencies including already installed ones or repo
	noConfirm   bool
	noDeps      bool      // If true, the graph will not include dependencies
	noCheckDeps bool      // If true, the graph will not include dependencies
	w           io.Writer // output writer

	providerCache map[string]*aur.Pkg
}

func NewGrapher(dbExecutor db.Executor, aurCache AURCache,
	fullGraph, noConfirm bool, output io.Writer, noDeps bool, noCheckDeps bool,
) *Grapher {
	return &Grapher{
		dbExecutor:    dbExecutor,
		aurCache:      aurCache,
		fullGraph:     fullGraph,
		noConfirm:     noConfirm,
		w:             output,
		noDeps:        noDeps,
		noCheckDeps:   noCheckDeps,
		providerCache: make(map[string]*aurc.Pkg, 5),
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

func (g *Grapher) GraphFromAURCache(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = topo.New[string, *InstallInfo]()
	}

	for _, target := range targets {
		aurPkgs, _ := g.aurCache.Get(ctx, &metadata.AURQuery{By: aurc.Name, Needles: []string{target}})
		if len(aurPkgs) == 0 {
			text.Errorln("No AUR package found for", target)

			continue
		}

		pkg := provideMenu(g.w, target, aurPkgs, g.noConfirm)

		graph.AddNode(pkg.Name)
		g.ValidateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
			Color:      colorMap[Explicit],
			Background: bgColorMap[AUR],
			Value: &InstallInfo{
				Source:  AUR,
				Reason:  Explicit,
				AURBase: &pkg.PackageBase,
				Version: pkg.Version,
			},
		})

		g.addDepNodes(ctx, pkg, graph)
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
					text.Warnln(depName, parentPkgName, err)
				}
			}

			continue
		}

		if graph.Exists(depName) {
			if err := graph.DependOn(depName, parentPkgName); err != nil {
				text.Warnln(depName, parentPkgName, err)
			}

			continue
		}

		// Check ALPM
		if alpmPkg := g.dbExecutor.SyncSatisfier(depString); alpmPkg != nil {
			if err := graph.DependOn(alpmPkg.Name(), parentPkgName); err != nil {
				text.Warnln("repo dep warn:", depName, parentPkgName, err)
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
			aurPkgs = []aur.Pkg{*cachedProvidePkg}
		} else {
			var errMeta error
			aurPkgs, errMeta = g.aurCache.Get(ctx,
				&metadata.AURQuery{
					Needles:  []string{depName},
					By:       aurc.None,
					Contains: false,
				})
			if errMeta != nil {
				text.Warnln("AUR cache error:", errMeta)
			}
		}

		if len(aurPkgs) != 0 { // Check AUR
			pkg := aurPkgs[0]
			if len(aurPkgs) > 1 {
				chosen := provideMenu(g.w, depName, aurPkgs, g.noConfirm)
				g.providerCache[depName] = chosen
				pkg = *chosen
			}

			if err := graph.DependOn(pkg.Name, parentPkgName); err != nil {
				text.Warnln("aur dep warn:", pkg.Name, parentPkgName, err)
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

func provideMenu(w io.Writer, dep string, options []aur.Pkg, noConfirm bool) *aur.Pkg {
	size := len(options)
	if size == 1 {
		return &options[0]
	}

	str := text.Bold(gotext.Get("There are %d providers available for %s:", size, dep))
	str += "\n"

	size = 1
	str += text.SprintOperationInfo(gotext.Get("Repository AUR"), "\n    ")

	for i := range options {
		str += fmt.Sprintf("%d) %s ", size, options[i].Name)
		size++
	}

	text.OperationInfoln(str)

	for {
		fmt.Fprintln(w, gotext.Get("\nEnter a number (default=1): "))

		if noConfirm {
			fmt.Fprintln(w, "1")

			return &options[0]
		}

		numberBuf, err := text.GetInput("", false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			break
		}

		if numberBuf == "" {
			return &options[0]
		}

		num, err := strconv.Atoi(numberBuf)
		if err != nil {
			text.Errorln(gotext.Get("invalid number: %s", numberBuf))

			continue
		}

		if num < 1 || num >= size {
			text.Errorln(gotext.Get("invalid value: %d is not between %d and %d", num, 1, size-1))

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

	for i := range srcInfo.Packages {
		pkg := &srcInfo.Packages[i]

		pkgs = append(pkgs, aur.Pkg{
			ID:            0,
			Name:          pkg.Pkgname,
			PackageBaseID: 0,
			PackageBase:   srcInfo.Pkgbase,
			Version:       srcInfo.Version(),
			Description:   pkg.Pkgdesc,
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

func AddUpgradeToGraph(pkg *db.Upgrade, graph *topo.Graph[string, *InstallInfo]) {
	source := Sync
	if pkg.Repository == "aur" {
		source = AUR
	}

	reason := Explicit
	if pkg.Reason == alpm.PkgReasonDepend {
		reason = Dep
	}

	graph.AddNode(pkg.Name)
	graph.SetNodeInfo(pkg.Name, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[reason],
		Background: bgColorMap[source],
		Value: &InstallInfo{
			Source:     source,
			Reason:     reason,
			Version:    pkg.RemoteVersion,
			AURBase:    &pkg.Base,
			SyncDBName: &pkg.Repository,
		},
	})
}
