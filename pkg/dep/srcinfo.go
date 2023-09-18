package dep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	aurc "github.com/Jguer/aur"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/dep/topo"
	"github.com/Jguer/yay/v12/pkg/intrange"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/text"
)

var ErrNoBuildFiles = errors.New(gotext.Get("cannot find PKGBUILD and .SRCINFO in directory"))

var _ SourceHandler = &SRCINFOHandler{}

type SRCINFOHandler struct {
	cfg          *settings.Configuration
	log          *text.Logger
	db           db.Executor
	cmdBuilder   exe.ICmdBuilder
	noConfirm    bool
	foundTargets []string

	aurHandler *AURHandler
}

func (g *SRCINFOHandler) Test(target Target) bool {
	path := filepath.Join(g.cfg.BuildDir, target.Name)
	if _, err := os.Stat(path); err == nil {
		g.foundTargets = append(g.foundTargets, path)
		return true
	}

	return false
}

func (g *SRCINFOHandler) Graph(ctx context.Context, graph *topo.Graph[string, *InstallInfo]) error {
	_, err := g.GraphFromSrcInfoDirs(ctx, graph, g.foundTargets)
	return err
}

func (g *SRCINFOHandler) GraphFromSrcInfoDirs(ctx context.Context, graph *topo.Graph[string, *InstallInfo],
	srcInfosDirs []string,
) (*topo.Graph[string, *InstallInfo], error) {
	if graph == nil {
		graph = NewGraph()
	}

	srcInfos := map[string]*gosrc.Srcinfo{}
	for _, targetDir := range srcInfosDirs {
		if err := srcinfoExists(ctx, g.cmdBuilder, targetDir); err != nil {
			return nil, err
		}

		pkgbuild, err := gosrc.ParseFile(filepath.Join(targetDir, ".SRCINFO"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", gotext.Get("failed to parse .SRCINFO"), err)
		}

		srcInfos[targetDir] = pkgbuild
	}

	aurPkgsAdded := []*aurc.Pkg{}
	for pkgBuildDir, pkgbuild := range srcInfos {
		pkgBuildDir := pkgBuildDir

		aurPkgs, err := makeAURPKGFromSrcinfo(g.db, pkgbuild)
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
			if pkg := g.db.LocalPackage(pkg.Name); pkg != nil {
				reason = Reason(pkg.Reason())
			}

			graph.AddNode(pkg.Name)

			g.aurHandler.AddAurPkgProvides(pkg, graph)

			validateAndSetNodeInfo(graph, pkg.Name, &topo.NodeInfo[*InstallInfo]{
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

	g.aurHandler.AddDepsForPkgs(ctx, aurPkgsAdded, graph)

	return graph, nil
}

func srcinfoExists(ctx context.Context,
	cmdBuilder exe.ICmdBuilder, targetDir string,
) error {
	srcInfoDir := filepath.Join(targetDir, ".SRCINFO")
	pkgbuildDir := filepath.Join(targetDir, "PKGBUILD")
	if _, err := os.Stat(srcInfoDir); err == nil {
		if _, err := os.Stat(pkgbuildDir); err == nil {
			return nil
		}
	}

	if _, err := os.Stat(pkgbuildDir); err == nil {
		// run makepkg to generate .SRCINFO
		srcinfo, stderr, err := cmdBuilder.Capture(cmdBuilder.BuildMakepkgCmd(ctx, targetDir, "--printsrcinfo"))
		if err != nil {
			return fmt.Errorf("unable to generate .SRCINFO: %w - %s", err, stderr)
		}

		if err := os.WriteFile(srcInfoDir, []byte(srcinfo), 0o600); err != nil {
			return fmt.Errorf("unable to write .SRCINFO: %w", err)
		}

		return nil
	}

	return fmt.Errorf("%w: %s", ErrNoBuildFiles, targetDir)
}

func (g *SRCINFOHandler) pickSrcInfoPkgs(pkgs []*aurc.Pkg) ([]*aurc.Pkg, error) {
	final := make([]*aurc.Pkg, 0, len(pkgs))
	for i := range pkgs {
		g.log.Println(text.Magenta(strconv.Itoa(i+1)+" ") + text.Bold(pkgs[i].Name) +
			" " + text.Cyan(pkgs[i].Version))
		g.log.Println("    " + pkgs[i].Description)
	}
	g.log.Infoln(gotext.Get("Packages to exclude") + " (eg: \"1 2 3\", \"1-3\", \"^4\"):")

	numberBuf, err := g.log.GetInput("", g.noConfirm)
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
