package workdir

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/runtime"
	settingslua "github.com/Jguer/yay/v13/pkg/settings/lua"

	gosrc "github.com/Morganamilo/go-srcinfo"
	mapset "github.com/deckarep/golang-set/v2"
)

func (preper *Preparer) runPreDownloadSourcesHooks(ctx context.Context, run *runtime.Runtime, w io.Writer,
	pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
	targets []map[string]*dep.InstallInfo,
) error {
	if err := runAURPreInstallLuaHooks(run, pkgbuildDirsByBase, installed, targets); err != nil {
		return err
	}

	for _, hookFn := range preper.hooks {
		if hookFn.Type != PreDownloadSourcesHook {
			continue
		}

		if err := hookFn.Hookfn(ctx, run, w, pkgbuildDirsByBase, installed); err != nil {
			return err
		}
	}

	return nil
}

func runAURPreInstallLuaHooks(run *runtime.Runtime, pkgbuildDirsByBase map[string]string,
	installed mapset.Set[string], targets []map[string]*dep.InstallInfo,
) error {
	if run == nil || run.Lua == nil || !run.Lua.HasAutocmd(settingslua.EventAURPreInstall) {
		return nil
	}

	events, err := aurPreInstallEvents(pkgbuildDirsByBase, installed, targets)
	if err != nil {
		return err
	}

	for i := range events {
		if err := run.Lua.RunAURPreInstall(&events[i]); err != nil {
			return err
		}
	}

	return nil
}

func aurPreInstallEvents(pkgbuildDirsByBase map[string]string, installed mapset.Set[string],
	targets []map[string]*dep.InstallInfo,
) ([]settingslua.AURPreInstallEvent, error) {
	packagesByBase := aurTargetPackagesByBase(targets)
	bases := sortedAURBases(pkgbuildDirsByBase)

	events := make([]settingslua.AURPreInstallEvent, 0, len(bases))
	for _, base := range bases {
		event, err := aurPreInstallEvent(base, pkgbuildDirsByBase[base],
			packagesByBase[base], installed, targets)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

func sortedAURBases(pkgbuildDirsByBase map[string]string) []string {
	return slices.Sorted(maps.Keys(pkgbuildDirsByBase))
}

func aurPreInstallEvent(base, path string, packages []settingslua.AURPreInstallPackage,
	installed mapset.Set[string], targets []map[string]*dep.InstallInfo,
) (settingslua.AURPreInstallEvent, error) {
	return aurPackageEvent(settingslua.EventAURPreInstall, base, path, packages, installed, targets)
}

func aurPackageEvent(eventName, base, path string, packages []settingslua.AURPreInstallPackage,
	installed mapset.Set[string], targets []map[string]*dep.InstallInfo,
) (settingslua.AURPreInstallEvent, error) {
	dir, pkgbuildPath, srcinfoPath := aurPreInstallPaths(path)

	pkgbuildBytes, err := os.ReadFile(pkgbuildPath)
	if err != nil {
		return settingslua.AURPreInstallEvent{},
			fmt.Errorf("%s %s: read PKGBUILD: %w", eventName, base, err)
	}

	srcinfo, err := gosrc.ParseFile(srcinfoPath)
	if err != nil {
		return settingslua.AURPreInstallEvent{},
			fmt.Errorf("%s %s: parse .SRCINFO: %w", eventName, base, err)
	}

	if len(packages) == 0 {
		packages = packagesFromSRCINFO(srcinfo)
	}
	slices.SortFunc(packages, func(a, b settingslua.AURPreInstallPackage) int { return cmp.Compare(a.Name, b.Name) })

	return settingslua.AURPreInstallEvent{
		Base:         base,
		Dir:          dir,
		PKGBUILDPath: pkgbuildPath,
		SRCINFOPath:  srcinfoPath,
		PKGBUILD:     string(pkgbuildBytes),
		Version:      srcinfo.Version(),
		LastModified: aurPreInstallLastModified(base, targets),
		Installed:    aurPreInstallInstalled(packages, srcinfo, installed),
		Packages:     packages,
		SRCINFO:      srcinfoEventData(srcinfo),
	}, nil
}

func aurPreInstallPaths(path string) (dir, pkgbuildPath, srcinfoPath string) {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
		srcinfoPath = path
		pkgbuildPath = filepath.Join(dir, "PKGBUILD")

		return dir, pkgbuildPath, srcinfoPath
	}

	dir = path
	pkgbuildPath = filepath.Join(dir, "PKGBUILD")
	srcinfoPath = filepath.Join(dir, ".SRCINFO")

	return dir, pkgbuildPath, srcinfoPath
}

func aurTargetPackagesByBase(targets []map[string]*dep.InstallInfo) map[string][]settingslua.AURPreInstallPackage {
	packages := map[string][]settingslua.AURPreInstallPackage{}

	for _, layer := range targets {
		for name, info := range layer {
			if info == nil || info.AURBase == "" {
				continue
			}

			if info.Source != dep.AUR && info.Source != dep.SrcInfo {
				continue
			}

			base := info.AURBase
			packages[base] = append(packages[base], settingslua.AURPreInstallPackage{
				Name:         name,
				Version:      info.Version,
				LocalVersion: info.LocalVersion,
				Reason:       luaReason(info.Reason),
				Upgrade:      info.Upgrade,
				Devel:        info.Devel,
			})
		}
	}

	return packages
}

func packagesFromSRCINFO(srcinfo *gosrc.Srcinfo) []settingslua.AURPreInstallPackage {
	packages := make([]settingslua.AURPreInstallPackage, 0, len(srcinfo.Packages))
	for i := range srcinfo.Packages {
		packages = append(packages, settingslua.AURPreInstallPackage{
			Name:    srcinfo.Packages[i].Pkgname,
			Version: srcinfo.Version(),
		})
	}

	return packages
}

func aurPreInstallInstalled(packages []settingslua.AURPreInstallPackage, srcinfo *gosrc.Srcinfo,
	installed mapset.Set[string],
) bool {
	isInstalled := false
	for _, pkg := range packages {
		if pkg.LocalVersion != "" || installed.Contains(pkg.Name) {
			isInstalled = true
			break
		}
	}

	for i := range srcinfo.Packages {
		if installed.Contains(srcinfo.Packages[i].Pkgname) {
			isInstalled = true
			break
		}
	}

	return isInstalled
}

func aurPreInstallLastModified(base string, targets []map[string]*dep.InstallInfo) int64 {
	var lastModified int64

	for _, layer := range targets {
		for _, info := range layer {
			if info == nil || info.AURBase == "" {
				continue
			}

			if info.AURBase == base && info.LastModified > lastModified {
				lastModified = info.LastModified
			}
		}
	}

	return lastModified
}

func luaReason(reason dep.Reason) string {
	switch reason {
	case dep.Explicit:
		return "explicit"
	case dep.Dep:
		return "dependency"
	case dep.MakeDep:
		return "make_dependency"
	case dep.CheckDep:
		return "check_dependency"
	default:
		return "unknown"
	}
}

func srcinfoEventData(srcinfo *gosrc.Srcinfo) settingslua.AURPreInstallSRCINFO {
	pkgdesc, url := srcinfoDescriptionAndURL(srcinfo)
	packageArch := packageStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []string {
		return pkg.Arch
	})
	packageLicense := packageStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []string {
		return pkg.License
	})
	packageDepends := packageArchStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []gosrc.ArchString {
		return pkg.Depends
	})
	packageOptDepends := packageArchStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []gosrc.ArchString {
		return pkg.OptDepends
	})
	packageProvides := packageArchStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []gosrc.ArchString {
		return pkg.Provides
	})
	packageConflicts := packageArchStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []gosrc.ArchString {
		return pkg.Conflicts
	})
	packageReplaces := packageArchStringValues(srcinfo.Packages, func(pkg *gosrc.Package) []gosrc.ArchString {
		return pkg.Replaces
	})

	return settingslua.AURPreInstallSRCINFO{
		Pkgbase:      srcinfo.Pkgbase,
		Pkgver:       srcinfo.Pkgver,
		Pkgrel:       srcinfo.Pkgrel,
		Epoch:        srcinfo.Epoch,
		Version:      srcinfo.Version(),
		Pkgdesc:      pkgdesc,
		URL:          url,
		Arch:         mergedStringValues(srcinfo.Arch, packageArch...),
		License:      mergedStringValues(srcinfo.License, packageLicense...),
		Depends:      mergedArchStringValues(srcinfo.Depends, packageDepends...),
		MakeDepends:  archStringValues(srcinfo.MakeDepends),
		CheckDepends: archStringValues(srcinfo.CheckDepends),
		OptDepends:   mergedArchStringValues(srcinfo.OptDepends, packageOptDepends...),
		Provides:     mergedArchStringValues(srcinfo.Provides, packageProvides...),
		Conflicts:    mergedArchStringValues(srcinfo.Conflicts, packageConflicts...),
		Replaces:     mergedArchStringValues(srcinfo.Replaces, packageReplaces...),
	}
}

func srcinfoDescriptionAndURL(srcinfo *gosrc.Srcinfo) (pkgdesc, url string) {
	pkgdesc = srcinfo.Pkgdesc
	url = srcinfo.URL
	if pkgdesc != "" && url != "" {
		return pkgdesc, url
	}

	for i := range srcinfo.Packages {
		pkg := &srcinfo.Packages[i]
		if pkgdesc == "" {
			pkgdesc = pkg.Pkgdesc
		}
		if url == "" {
			url = pkg.URL
		}
		if pkgdesc != "" && url != "" {
			break
		}
	}

	return pkgdesc, url
}

func packageStringValues(packages []gosrc.Package, selectValues func(*gosrc.Package) []string) [][]string {
	values := make([][]string, 0, len(packages))
	for i := range packages {
		values = append(values, selectValues(&packages[i]))
	}

	return values
}

func packageArchStringValues(packages []gosrc.Package, selectValues func(*gosrc.Package) []gosrc.ArchString) [][]gosrc.ArchString {
	values := make([][]gosrc.ArchString, 0, len(packages))
	for i := range packages {
		values = append(values, selectValues(&packages[i]))
	}

	return values
}

func mergedStringValues(global []string, packageValues ...[]string) []string {
	out := make([]string, 0, len(global))
	seen := map[string]struct{}{}

	appendValue := func(value string) {
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	for _, value := range global {
		appendValue(value)
	}
	for _, values := range packageValues {
		for _, value := range values {
			appendValue(value)
		}
	}

	return out
}

func mergedArchStringValues(global []gosrc.ArchString, packageValues ...[]gosrc.ArchString) []string {
	values := make([][]string, 0, len(packageValues))
	for _, packageValue := range packageValues {
		values = append(values, archStringValues(packageValue))
	}

	return mergedStringValues(archStringValues(global), values...)
}

func archStringValues(values []gosrc.ArchString) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Value)
	}

	return out
}
