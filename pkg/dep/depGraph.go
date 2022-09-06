package dep

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/metadata"
	aur "github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/text"
	"github.com/Jguer/yay/v11/pkg/topo"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
)

type Grapher struct {
	dbExecutor db.Executor
	aurCache   *metadata.AURCache
	fullGraph  bool // If true, the graph will include all dependencies including already installed ones or repo
	noConfirm  bool
	w          io.Writer // output writer
}

func NewGrapher(dbExecutor db.Executor, aurCache *metadata.AURCache, fullGraph, noConfirm bool, output io.Writer) *Grapher {
	return &Grapher{
		dbExecutor: dbExecutor,
		aurCache:   aurCache,
		fullGraph:  fullGraph,
		noConfirm:  noConfirm,
		w:          output,
	}
}

func (g *Grapher) GraphFromSrcInfo(pkgbuild *gosrc.Srcinfo) (*topo.Graph[string], error) {
	graph := topo.New[string]()

	aurPkgs, err := makeAURPKGFromSrcinfo(g.dbExecutor, pkgbuild)
	if err != nil {
		return nil, err
	}

	for _, pkg := range aurPkgs {
		pkg := pkg
		depSlice := ComputeCombinedDepList(&pkg, false, false)
		g.addNodes(graph, pkg.Name, depSlice)
	}

	return graph, nil
}

func (g *Grapher) GraphFromAURCache(targets []string) (*topo.Graph[string], error) {
	graph := topo.New[string]()

	for _, target := range targets {
		aurPkgs, _ := g.aurCache.FindPackage(target)
		pkg := provideMenu(g.w, target, aurPkgs, g.noConfirm)

		depSlice := ComputeCombinedDepList(pkg, false, false)
		g.addNodes(graph, pkg.Name, depSlice)
	}

	return graph, nil
}

func (g *Grapher) addNodes(
	graph *topo.Graph[string],
	parentPkgName string,
	deps []string,
) {
	for _, depString := range deps {
		depName, _, _ := splitDep(depString)

		if g.dbExecutor.LocalSatisfierExists(depString) {
			if g.fullGraph {
				graph.SetNodeInfo(depName, &topo.NodeInfo{Color: "green"})
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

			graph.SetNodeInfo(alpmPkg.Name(), &topo.NodeInfo{Color: "blue"})

			if newDeps := alpmPkg.Depends().Slice(); len(newDeps) != 0 && g.fullGraph {
				newDepsSlice := make([]string, 0, len(newDeps))
				for _, newDep := range newDeps {
					newDepsSlice = append(newDepsSlice, newDep.Name)
				}

				g.addNodes(graph, alpmPkg.Name(), newDepsSlice)
			}

			continue
		}

		if aurPkgs, _ := g.aurCache.FindPackage(depName); len(aurPkgs) != 0 { // Check AUR
			pkg := aurPkgs[0]
			if len(aurPkgs) > 1 {
				pkg := provideMenu(g.w, depName, aurPkgs, g.noConfirm)
				g.aurCache.SetProvideCache(depName, []*aur.Pkg{pkg})
			}

			if err := graph.Alias(pkg.PackageBase, pkg.Name); err != nil {
				text.Warnln("aur alias warn:", pkg.PackageBase, pkg.Name, err)
			}

			if err := graph.DependOn(pkg.PackageBase, parentPkgName); err != nil {
				text.Warnln("aur dep warn:", pkg.PackageBase, parentPkgName, err)
			}

			graph.SetNodeInfo(pkg.PackageBase, &topo.NodeInfo{Color: "lightgreen"})

			if newDeps := ComputeCombinedDepList(pkg, false, false); len(newDeps) != 0 {
				g.addNodes(graph, pkg.Name, newDeps)
			}

			continue
		}
	}
}

func provideMenu(w io.Writer, dep string, options []*aur.Pkg, noConfirm bool) *aur.Pkg {
	size := len(options)
	if size == 1 {
		return options[0]
	}

	str := text.Bold(gotext.Get("There are %d providers available for %s:", size, dep))
	str += "\n"

	size = 1
	str += text.SprintOperationInfo(gotext.Get("Repository AUR"), "\n    ")

	for _, pkg := range options {
		str += fmt.Sprintf("%d) %s ", size, pkg.Name)
		size++
	}

	text.OperationInfoln(str)

	for {
		fmt.Fprintln(w, gotext.Get("\nEnter a number (default=1): "))

		if noConfirm {
			fmt.Fprintln(w, "1")

			return options[0]
		}

		numberBuf, err := text.GetInput("", false)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)

			break
		}

		if numberBuf == "" {
			return options[0]
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

		return options[num-1]
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

	for _, pkg := range srcInfo.Packages {
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
