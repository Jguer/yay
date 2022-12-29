package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	aur "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/query"
	"github.com/Jguer/yay/v11/pkg/settings"
	"github.com/Jguer/yay/v11/pkg/settings/parser"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

// SyncSearch presents a query to the local repos and to the AUR.
func syncSearch(ctx context.Context, pkgS []string,
	dbExecutor db.Executor, queryBuilder query.Builder, verbose bool,
) error {
	queryBuilder.Execute(ctx, dbExecutor, pkgS)

	searchMode := query.Minimal
	if verbose {
		searchMode = query.Detailed
	}

	return queryBuilder.Results(os.Stdout, dbExecutor, searchMode)
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func syncInfo(ctx context.Context, cmdArgs *parser.Arguments, pkgS []string, dbExecutor db.Executor) error {
	var (
		info    []aur.Pkg
		err     error
		missing = false
	)

	pkgS = query.RemoveInvalidTargets(pkgS, config.Runtime.Mode)
	aurS, repoS := packageSlices(pkgS, dbExecutor)

	if len(aurS) != 0 {
		noDB := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := text.SplitDBFromName(pkg)
			noDB = append(noDB, name)
		}

		info, err = query.AURInfoPrint(ctx, config.Runtime.AURClient, noDB, config.RequestSplitN)
		if err != nil {
			missing = true

			fmt.Fprintln(os.Stderr, err)
		}
	}

	// Repo always goes first
	if len(repoS) != 0 {
		arguments := cmdArgs.Copy()
		arguments.ClearTargets()
		arguments.AddTarget(repoS...)

		err = config.Runtime.CmdBuilder.Show(config.Runtime.CmdBuilder.BuildPacmanCmd(ctx,
			cmdArgs, config.Runtime.Mode, settings.NoConfirm))
		if err != nil {
			return err
		}
	}

	if len(aurS) != len(info) {
		missing = true
	}

	if len(info) != 0 {
		for i := range info {
			PrintInfo(&info[i], cmdArgs.ExistsDouble("i"))
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return err
}

// PackageSlices separates an input slice into aur and repo slices.
func packageSlices(toCheck []string, dbExecutor db.Executor) (aurNames, repoNames []string) {
	for _, _pkg := range toCheck {
		dbName, name := text.SplitDBFromName(_pkg)

		if dbName == "aur" || config.Runtime.Mode == parser.ModeAUR {
			aurNames = append(aurNames, _pkg)
			continue
		} else if dbName != "" || config.Runtime.Mode == parser.ModeRepo {
			repoNames = append(repoNames, _pkg)
			continue
		}

		if dbExecutor.SyncSatisfierExists(name) ||
			len(dbExecutor.PackagesFromGroup(name)) != 0 {
			repoNames = append(repoNames, _pkg)
		} else {
			aurNames = append(aurNames, _pkg)
		}
	}

	return aurNames, repoNames
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
// removeOptional decides whether optional dependencies are counted or not.
func hangingPackages(removeOptional bool, dbExecutor db.Executor) (hanging []string) {
	// safePackages represents every package in the system in one of 3 states
	// State = 0 - Remove package from the system
	// State = 1 - Keep package in the system; need to iterate over dependencies
	// State = 2 - Keep package and have iterated over dependencies
	safePackages := make(map[string]uint8)
	// provides stores a mapping from the provides name back to the original package name
	provides := make(stringset.MapStringSet)

	packages := dbExecutor.LocalPackages()
	// Mark explicit dependencies and enumerate the provides list
	for _, pkg := range packages {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			safePackages[pkg.Name()] = 1
		} else {
			safePackages[pkg.Name()] = 0
		}

		for _, dep := range dbExecutor.PackageProvides(pkg) {
			provides.Add(dep.Name, pkg.Name())
		}
	}

	iterateAgain := true

	for iterateAgain {
		iterateAgain = false

		for _, pkg := range packages {
			if state := safePackages[pkg.Name()]; state == 0 || state == 2 {
				continue
			}

			safePackages[pkg.Name()] = 2
			deps := dbExecutor.PackageDepends(pkg)

			if !removeOptional {
				deps = append(deps, dbExecutor.PackageOptionalDepends(pkg)...)
			}

			// Update state for dependencies
			for _, dep := range deps {
				// Don't assume a dependency is installed
				state, ok := safePackages[dep.Name]
				if !ok {
					// Check if dep is a provides rather than actual package name
					if pset, ok2 := provides[dep.Name]; ok2 {
						for p := range pset {
							if safePackages[p] == 0 {
								iterateAgain = true
								safePackages[p] = 1
							}
						}
					}

					continue
				}

				if state == 0 {
					iterateAgain = true
					safePackages[dep.Name] = 1
				}
			}
		}
	}

	// Build list of packages to be removed
	for _, pkg := range packages {
		if safePackages[pkg.Name()] == 0 {
			hanging = append(hanging, pkg.Name())
		}
	}

	return hanging
}

func getFolderSize(path string) (size int64) {
	_ = filepath.WalkDir(path, func(p string, entry fs.DirEntry, err error) error {
		info, _ := entry.Info()
		size += info.Size()
		return nil
	})

	return size
}

// Statistics returns statistics about packages installed in system.
func statistics(dbExecutor db.Executor) (res struct {
	Totaln       int
	Expln        int
	TotalSize    int64
	pacmanCaches map[string]int64
	yayCache     int64
},
) {
	for _, pkg := range dbExecutor.LocalPackages() {
		res.TotalSize += pkg.ISize()
		res.Totaln++

		if pkg.Reason() == alpm.PkgReasonExplicit {
			res.Expln++
		}
	}

	res.pacmanCaches = make(map[string]int64)
	for _, path := range config.Runtime.PacmanConf.CacheDir {
		res.pacmanCaches[path] = getFolderSize(path)
	}

	res.yayCache = getFolderSize(config.BuildDir)

	return
}
