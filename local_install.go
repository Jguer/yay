package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/aur"
	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/metadata"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/topo"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"
)

func archStringToString(alpmArches []string, archString []gosrc.ArchString) []string {
	pkgs := make([]string, 0, len(archString))

	for _, arch := range archString {
		if alpmArchIsSupported(alpmArches, arch.Arch) {
			pkgs = append(pkgs, arch.Value)
		}
	}

	return pkgs
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
			Depends:       append(archStringToString(alpmArch, pkg.Depends), archStringToString(alpmArch, srcInfo.Package.Depends)...),
			MakeDepends:   archStringToString(alpmArch, srcInfo.PackageBase.MakeDepends),
			CheckDepends:  archStringToString(alpmArch, srcInfo.PackageBase.CheckDepends),
			Conflicts:     append(archStringToString(alpmArch, pkg.Conflicts), archStringToString(alpmArch, srcInfo.Package.Conflicts)...),
			Provides:      append(archStringToString(alpmArch, pkg.Provides), archStringToString(alpmArch, srcInfo.Package.Provides)...),
			Replaces:      append(archStringToString(alpmArch, pkg.Replaces), archStringToString(alpmArch, srcInfo.Package.Replaces)...),
			OptDepends:    []string{},
			Groups:        pkg.Groups,
			License:       pkg.License,
			Keywords:      []string{},
		})
	}

	return pkgs, nil
}

func splitDep(dep string) (pkg, mod, ver string) {
	split := strings.FieldsFunc(dep, func(c rune) bool {
		match := c == '>' || c == '<' || c == '='

		if match {
			mod += string(c)
		}

		return match
	})

	if len(split) == 0 {
		return "", "", ""
	}

	if len(split) == 1 {
		return split[0], "", ""
	}

	return split[0], mod, split[1]
}

func installLocalPKGBUILD(
	ctx context.Context,
	cmdArgs *parser.Arguments,
	dbExecutor db.Executor,
	ignoreProviders bool,
) error {
	aurCache, err := metadata.NewAURCache("aur.json")
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve aur Cache"))
	}

	wd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to retrieve working directory"))
	}

	if len(cmdArgs.Targets) > 1 {
		return errors.New(gotext.Get("only one target is allowed"))
	}

	if len(cmdArgs.Targets) == 1 {
		wd = cmdArgs.Targets[0]
	}

	pkgbuild, err := gosrc.ParseFile(filepath.Join(wd, ".SRCINFO"))
	if err != nil {
		return errors.Wrap(err, gotext.Get("failed to parse .SRCINFO"))
	}

	aurPkgs, err := makeAURPKGFromSrcinfo(dbExecutor, pkgbuild)
	if err != nil {
		return err
	}

	graph := topo.New[string]()

	for _, pkg := range aurPkgs {
		depSlice := dep.ComputeCombinedDepList(&pkg, false, false)
		addNodes(dbExecutor, aurCache, pkg.Name, pkg.PackageBase, depSlice, graph)
	}

	// fmt.Println(graph)
	// aurCache.DebugInfo()

	// topoSorted := graph.TopoSortedLayers()
	// fmt.Println(topoSorted, len(topoSorted))

	return nil
}

func addNodes(dbExecutor db.Executor, aurCache *metadata.AURCache, pkgName string, pkgBase string, deps []string, graph *topo.Graph[string]) {
	graph.AddNode(pkgBase)
	graph.Alias(pkgBase, pkgName)
	graph.SetNodeInfo(pkgBase, &topo.NodeInfo{Color: "blue"})

	for _, depString := range deps {
		depName, _, _ := splitDep(depString)

		if dbExecutor.LocalSatisfierExists(depString) {
			continue
		} else if graph.Exists(depName) {
			graph.DependOn(depName, pkgBase)
			continue
		}

		graph.DependOn(depName, pkgBase)

		// Check ALPM
		if alpmPkg := dbExecutor.SyncSatisfier(depString); alpmPkg != nil {
			newDeps := alpmPkg.Depends().Slice()
			newDepsSlice := make([]string, 0, len(newDeps))

			for _, newDep := range newDeps {
				newDepsSlice = append(newDepsSlice, newDep.Name)
			}

			if len(newDeps) == 0 {
				continue
			}

			addNodes(dbExecutor, aurCache, alpmPkg.Name(), alpmPkg.Base(), newDepsSlice, graph)
			// Check AUR
		} else if aurPkgs, _ := aurCache.FindDep(depName); len(aurPkgs) != 0 {
			pkg := aurPkgs[0]
			newDeps := dep.ComputeCombinedDepList(pkg, false, false)

			if len(newDeps) == 0 {
				continue
			}

			addNodes(dbExecutor, aurCache, pkg.PackageBase, pkg.Name, newDeps, graph)
		}
	}
}
