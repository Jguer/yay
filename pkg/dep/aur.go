package dep

import (
	"context"
	"fmt"
	"strconv"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

type AURHandler struct {
	log      *text.Logger
	db       db.Executor
	cfg      *settings.Configuration
	vcsStore vcs.Store

	foundPkgs []string

	providerCache map[string][]aur.Pkg
	dbExecutor    db.Executor
	aurClient     aurc.QueryClient
	fullGraph     bool // If true, the graph will include all dependencies including already installed ones or repo
	noConfirm     bool // If true, the graph will not prompt for confirmation
	noDeps        bool // If true, the graph will not include dependencies
	noCheckDeps   bool // If true, the graph will not include check dependencies
	needed        bool // If true, the graph will only include packages that are not installed
}

func (h *AURHandler) Test(target Target) bool {
	// FIXME: add test
	h.foundPkgs = append(h.foundPkgs, target.Name)
	return true
}

func (h *AURHandler) Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error {
	var errA error
	_, errA = h.GraphFromAUR(ctx, graph, h.foundPkgs)
	if errA != nil {
		return errA
	}
	return nil
}

func (h *AURHandler) AddDepsForPkgs(ctx context.Context, pkgs []*aur.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	for _, pkg := range pkgs {
		h.addDepNodes(ctx, pkg, graph)
	}
}

func (h *AURHandler) addDepNodes(ctx context.Context, pkg *aur.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	if len(pkg.MakeDepends) > 0 {
		h.addNodes(ctx, graph, pkg.Name, pkg.MakeDepends, MakeDep)
	}

	if !h.noDeps && len(pkg.Depends) > 0 {
		h.addNodes(ctx, graph, pkg.Name, pkg.Depends, Dep)
	}

	if !h.noCheckDeps && !h.noDeps && len(pkg.CheckDepends) > 0 {
		h.addNodes(ctx, graph, pkg.Name, pkg.CheckDepends, CheckDep)
	}
}

func (h *AURHandler) GraphAURTarget(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	pkg *aurc.Pkg, instalInfo *InstallInfo,
) *topo.Graph[string, *InstallInfo] {
	if graph == nil {
		graph = NewGraph()
	}

	graph.AddNode(pkg.Name)

	h.AddAurPkgProvides(pkg, graph)

	validateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
		Color:      colorMap[instalInfo.Reason],
		Background: bgColorMap[AUR],
		Value:      instalInfo,
	})

	return graph
}

func (h *AURHandler) GraphFromAUR(ctx context.Context,
	graph *topo.Graph[string, *InstallInfo],
	targets []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	if len(targets) == 0 {
		return graph, nil
	}

	aurPkgs, errCache := h.aurClient.Get(ctx, &aurc.Query{By: aurc.Name, Needles: targets})
	if errCache != nil {
		h.log.Errorln(errCache)
	}

	for i := range aurPkgs {
		pkg := &aurPkgs[i]
		if _, ok := h.providerCache[pkg.Name]; !ok {
			h.providerCache[pkg.Name] = []aurc.Pkg{*pkg}
		}
	}

	aurPkgsAdded := []*aurc.Pkg{}

	for _, target := range targets {
		if cachedProvidePkg, ok := h.providerCache[target]; ok {
			aurPkgs = cachedProvidePkg
		} else {
			var errA error
			aurPkgs, errA = h.aurClient.Get(ctx, &aurc.Query{By: aurc.Provides, Needles: []string{target}, Contains: true})
			if errA != nil {
				h.log.Errorln(gotext.Get("Failed to find AUR package for"), " ", target, ":", errA)
			}
		}

		if len(aurPkgs) == 0 {
			h.log.Errorln(gotext.Get("No AUR package found for"), " ", target)

			continue
		}

		aurPkg := &aurPkgs[0]
		if len(aurPkgs) > 1 {
			chosen := h.provideMenu(target, aurPkgs)
			aurPkg = chosen
			h.providerCache[target] = []aurc.Pkg{*aurPkg}
		}

		reason := Explicit
		if pkg := h.dbExecutor.LocalPackage(aurPkg.Name); pkg != nil {
			reason = Reason(pkg.Reason())

			if h.needed {
				if db.VerCmp(pkg.Version(), aurPkg.Version) >= 0 {
					h.log.Warnln(gotext.Get("%s is up to date -- skipping", text.Cyan(pkg.Name()+"-"+pkg.Version())))
					continue
				}
			}
		}

		graph = h.GraphAURTarget(ctx, graph, aurPkg, &InstallInfo{
			AURBase: &aurPkg.PackageBase,
			Reason:  reason,
			Source:  AUR,
			Version: aurPkg.Version,
		})
		aurPkgsAdded = append(aurPkgsAdded, aurPkg)
	}

	h.AddDepsForPkgs(ctx, aurPkgsAdded, graph)

	return graph, nil
}

func (h *AURHandler) AddAurPkgProvides(pkg *aurc.Pkg, graph *topo.Graph[string, *InstallInfo]) {
	for i := range pkg.Provides {
		depName, mod, version := splitDep(pkg.Provides[i])
		h.log.Debugln(pkg.String() + " provides: " + depName)
		graph.Provides(depName, &alpm.Depend{
			Name:    depName,
			Version: version,
			Mod:     aurDepModToAlpmDep(mod),
		}, pkg.Name)
	}
}

// Removes found deps from the deps mapset and returns the found deps.
func (h *AURHandler) findDepsFromAUR(ctx context.Context,
	deps mapset.Set[string],
) []aurc.Pkg {
	pkgsToAdd := make([]aurc.Pkg, 0, deps.Cardinality())
	if deps.Cardinality() == 0 {
		return []aurc.Pkg{}
	}

	missingNeedles := make([]string, 0, deps.Cardinality())
	for _, depString := range deps.ToSlice() {
		if _, ok := h.providerCache[depString]; !ok {
			depName, _, _ := splitDep(depString)
			missingNeedles = append(missingNeedles, depName)
		}
	}

	if len(missingNeedles) != 0 {
		h.log.Debugln("deps to find", missingNeedles)
		// provider search is more demanding than a simple search
		// try to find name match if possible and then try to find provides.
		aurPkgs, errCache := h.aurClient.Get(ctx, &aurc.Query{
			By: aurc.Name, Needles: missingNeedles, Contains: false,
		})
		if errCache != nil {
			h.log.Errorln(errCache)
		}

		for i := range aurPkgs {
			pkg := &aurPkgs[i]
			if deps.Contains(pkg.Name) {
				h.providerCache[pkg.Name] = append(h.providerCache[pkg.Name], *pkg)
			}

			for _, val := range pkg.Provides {
				if val == pkg.Name {
					continue
				}
				if deps.Contains(val) {
					h.providerCache[val] = append(h.providerCache[val], *pkg)
				}
			}
		}
	}

	for _, depString := range deps.ToSlice() {
		var aurPkgs []aurc.Pkg
		depName, _, _ := splitDep(depString)

		if cachedProvidePkg, ok := h.providerCache[depString]; ok {
			aurPkgs = cachedProvidePkg
		} else {
			var errA error
			aurPkgs, errA = h.aurClient.Get(ctx, &aurc.Query{By: aurc.Provides, Needles: []string{depName}, Contains: true})
			if errA != nil {
				h.log.Errorln(gotext.Get("Failed to find AUR package for"), depString, ":", errA)
			}
		}

		// remove packages that don't satisfy the dependency
		for i := 0; i < len(aurPkgs); i++ {
			if !satisfiesAur(depString, &aurPkgs[i]) {
				aurPkgs = append(aurPkgs[:i], aurPkgs[i+1:]...)
				i--
			}
		}

		if len(aurPkgs) == 0 {
			h.log.Errorln(gotext.Get("No AUR package found for"), " ", depString)

			continue
		}

		pkg := aurPkgs[0]
		if len(aurPkgs) > 1 {
			chosen := h.provideMenu(depString, aurPkgs)
			pkg = *chosen
		}

		h.providerCache[depString] = []aurc.Pkg{pkg}
		deps.Remove(depString)
		pkgsToAdd = append(pkgsToAdd, pkg)
	}

	return pkgsToAdd
}

func (h *AURHandler) addNodes(
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
				h.log.Warnln(depString, parentPkgName, err)
			}

			targetsToFind.Remove(depString)
		}

		if p := graph.GetProviderNode(depName); p != nil {
			if provideSatisfies(p.String(), depString, p.Version) {
				if err := graph.DependOn(p.Provider, parentPkgName); err != nil {
					h.log.Warnln(p.Provider, parentPkgName, err)
				}

				targetsToFind.Remove(depString)
			}
		}
	}

	// Check installed
	for _, depString := range targetsToFind.ToSlice() {
		depName, _, _ := splitDep(depString)
		if !h.dbExecutor.LocalSatisfierExists(depString) {
			continue
		}

		if h.fullGraph {
			validateAndSetNodeInfo(
				graph,
				depName,
				&topo.NodeInfo[*InstallInfo]{Color: colorMap[depType], Background: bgColorMap[Local]})

			if err := graph.DependOn(depName, parentPkgName); err != nil {
				h.log.Warnln(depName, parentPkgName, err)
			}
		}

		targetsToFind.Remove(depString)
	}

	// Check Sync
	for _, depString := range targetsToFind.ToSlice() {
		alpmPkg := h.dbExecutor.SyncSatisfier(depString)
		if alpmPkg == nil {
			continue
		}

		if err := graph.DependOn(alpmPkg.Name(), parentPkgName); err != nil {
			h.log.Warnln("repo dep warn:", depString, parentPkgName, err)
		}

		dbName := alpmPkg.DB().Name()
		validateAndSetNodeInfo(
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

		if newDeps := alpmPkg.Depends().Slice(); len(newDeps) != 0 && h.fullGraph {
			newDepsSlice := make([]string, 0, len(newDeps))
			for _, newDep := range newDeps {
				newDepsSlice = append(newDepsSlice, newDep.Name)
			}

			h.addNodes(ctx, graph, alpmPkg.Name(), newDepsSlice, Dep)
		}

		targetsToFind.Remove(depString)
	}

	// Check AUR
	pkgsToAdd := h.findDepsFromAUR(ctx, targetsToFind)
	for i := range pkgsToAdd {
		aurPkg := &pkgsToAdd[i]
		if err := graph.DependOn(aurPkg.Name, parentPkgName); err != nil {
			h.log.Warnln("aur dep warn:", aurPkg.Name, parentPkgName, err)
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

		h.addDepNodes(ctx, aurPkg, graph)
	}

	// Add missing to graph
	for _, depString := range targetsToFind.ToSlice() {
		depName, mod, ver := splitDep(depString)
		// no dep found. add as missing
		if err := graph.DependOn(depName, parentPkgName); err != nil {
			h.log.Warnln("missing dep warn:", depString, parentPkgName, err)
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

func (h *AURHandler) provideMenu(dep string, options []aur.Pkg) *aur.Pkg {
	size := len(options)
	if size == 1 {
		return &options[0]
	}

	str := text.Bold(gotext.Get("There are %d providers available for %s:", size, dep))
	str += "\n"

	size = 1
	str += h.log.SprintOperationInfo(gotext.Get("Repository AUR"), "\n    ")

	for i := range options {
		str += fmt.Sprintf("%d) %s ", size, options[i].Name)
		size++
	}

	h.log.OperationInfoln(str)

	for {
		h.log.Println(gotext.Get("\nEnter a number (default=1): "))

		if h.noConfirm {
			h.log.Println("1")

			return &options[0]
		}

		numberBuf, err := h.log.GetInput("", false)
		if err != nil {
			h.log.Errorln(err)

			break
		}

		if numberBuf == "" {
			return &options[0]
		}

		num, err := strconv.Atoi(numberBuf)
		if err != nil {
			h.log.Errorln(gotext.Get("invalid number: %s", numberBuf))

			continue
		}

		if num < 1 || num >= size {
			h.log.Errorln(gotext.Get("invalid value: %d is not between %d and %d",
				num, 1, size-1))

			continue
		}

		return &options[num-1]
	}

	return nil
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
