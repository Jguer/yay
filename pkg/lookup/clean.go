package lookup

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Morganamilo/go-pacmanconf"
)

// CleanDependencies removes all dangling dependencies in system
func CleanDependencies(config *runtime.Configuration, alpmHandle *alpm.Handle, pacmanConf *pacmanconf.Config, cmdArgs *types.Arguments, removeOptional bool) error {
	hanging, err := hangingPackages(alpmHandle, removeOptional)
	if err != nil {
		return err
	}

	if len(hanging) != 0 {
		return cleanRemove(config, pacmanConf, cmdArgs, hanging)
	}

	return nil
}

// SyncClean handles pacman -Sc wrapping
func SyncClean(config *runtime.Configuration, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, args *types.Arguments) error {
	var err error
	keepInstalled := false
	keepCurrent := false

	_, removeAll, _ := args.GetArg("c", "clean")

	for _, v := range pacmanConf.CleanMethod {
		if v == "KeepInstalled" {
			keepInstalled = true
		} else if v == "KeepCurrent" {
			keepCurrent = true
		}
	}

	if config.Mode.IsAnyOrRepo() {
		if err := exec.Show(exec.PassToPacman(config, pacmanConf, args, config.NoConfirm)); err != nil {
			return err
		}
	}

	if !config.Mode.IsAnyOrAUR() {
		return nil
	}

	var question string
	if removeAll {
		question = "Do you want to remove ALL AUR packages from cache?"
	} else {
		question = "Do you want to remove all other AUR packages from cache?"
	}

	fmt.Printf("\nBuild directory: %s\n", config.BuildDir)

	if text.ContinueTask(question, true, config.NoConfirm) {
		err = cleanAUR(config, alpmHandle, keepInstalled, keepCurrent, removeAll)
	}

	if err != nil || removeAll {
		return err
	}

	if text.ContinueTask("Do you want to remove ALL untracked AUR files?", true, config.NoConfirm) {
		return cleanUntracked(config)
	}

	return nil
}

func cleanAUR(config *runtime.Configuration, alpmHandle *alpm.Handle, keepInstalled, keepCurrent, removeAll bool) error {
	fmt.Println("removing AUR packages from cache...")

	installedBases := make(types.StringSet)
	inAURBases := make(types.StringSet)

	_, remotePackages, _, _, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	files, err := ioutil.ReadDir(config.BuildDir)
	if err != nil {
		return err
	}

	cachedPackages := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		cachedPackages = append(cachedPackages, file.Name())
	}

	// Most people probably don't use keep current and that is the only
	// case where this is needed.
	// Querying the AUR is slow and needs internet so don't do it if we
	// don't need to.
	if keepCurrent {
		info, err := query.AURInfo(config, cachedPackages, &types.AURWarnings{})
		if err != nil {
			return err
		}

		for _, pkg := range info {
			inAURBases.Set(pkg.PackageBase)
		}
	}

	for _, pkg := range remotePackages {
		if pkg.Base() != "" {
			installedBases.Set(pkg.Base())
		} else {
			installedBases.Set(pkg.Name())
		}
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		if !removeAll {
			if keepInstalled && installedBases.Get(file.Name()) {
				continue
			}

			if keepCurrent && inAURBases.Get(file.Name()) {
				continue
			}
		}

		err = os.RemoveAll(filepath.Join(config.BuildDir, file.Name()))
		if err != nil {
			return nil
		}
	}

	return nil
}

// cleanRemove sends a full removal command to pacman with the pkgName slice
func cleanRemove(config *runtime.Configuration, pacmanConf *pacmanconf.Config, cmdArgs *types.Arguments, pkgNames []string) (err error) {
	if len(pkgNames) == 0 {
		return nil
	}

	arguments := cmdArgs.CopyGlobal()
	arguments.AddArg("R")
	arguments.AddTarget(pkgNames...)

	return exec.Show(exec.PassToPacman(config, pacmanConf, arguments, config.NoConfirm))
}

func cleanUntracked(config *runtime.Configuration) error {
	fmt.Println("removing Untracked AUR files from cache...")

	files, err := ioutil.ReadDir(config.BuildDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		dir := filepath.Join(config.BuildDir, file.Name())
		if exec.ShouldUseGit(dir, config.GitClone) {
			if err := exec.Show(exec.PassToGit(config.GitBin, config.GitFlags, dir, "clean", "-fx")); err != nil {
				return err
			}
		}
	}

	return nil
}

// HangingPackages returns a list of packages installed as deps
// and unneeded by the system
// removeOptional decides whether optional dependencies are counted or not
func hangingPackages(alpmHandle *alpm.Handle, removeOptional bool) (hanging []string, err error) {
	localDB, err := alpmHandle.LocalDB()
	if err != nil {
		return
	}

	// safePackages represents every package in the system in one of 3 states
	// State = 0 - Remove package from the system
	// State = 1 - Keep package in the system; need to iterate over dependencies
	// State = 2 - Keep package and have iterated over dependencies
	safePackages := make(map[string]uint8)
	// provides stores a mapping from the provides name back to the original package name
	provides := make(types.MapStringSet)
	packages := localDB.PkgCache()

	// Mark explicit dependencies and enumerate the provides list
	setupResources := func(pkg alpm.Package) error {
		if pkg.Reason() == alpm.PkgReasonExplicit {
			safePackages[pkg.Name()] = 1
		} else {
			safePackages[pkg.Name()] = 0
		}

		pkg.Provides().ForEach(func(dep alpm.Depend) error {
			provides.Add(dep.Name, pkg.Name())
			return nil
		})
		return nil
	}
	packages.ForEach(setupResources)

	iterateAgain := true
	processDependencies := func(pkg alpm.Package) error {
		if state := safePackages[pkg.Name()]; state == 0 || state == 2 {
			return nil
		}

		safePackages[pkg.Name()] = 2

		// Update state for dependencies
		markDependencies := func(dep alpm.Depend) error {
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

				return nil
			}

			if state == 0 {
				iterateAgain = true
				safePackages[dep.Name] = 1
			}
			return nil
		}

		pkg.Depends().ForEach(markDependencies)
		if !removeOptional {
			pkg.OptionalDepends().ForEach(markDependencies)
		}
		return nil
	}

	for iterateAgain {
		iterateAgain = false
		packages.ForEach(processDependencies)
	}

	// Build list of packages to be removed
	packages.ForEach(func(pkg alpm.Package) error {
		if safePackages[pkg.Name()] == 0 {
			hanging = append(hanging, pkg.Name())
		}
		return nil
	})

	return
}
