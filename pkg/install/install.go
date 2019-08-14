package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/dep"
	"github.com/Jguer/yay/v10/pkg/download"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/install/pgp"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/lookup/upgrade"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/runtime/completion"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
	gosrc "github.com/Morganamilo/go-srcinfo"
)

const gitEmptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func inRepos(config *runtime.Configuration, syncDB alpm.DBList, pkg string) bool {
	target := dep.ToTarget(pkg)

	if target.DB == "aur" {
		return false
	} else if target.DB != "" {
		return true
	}

	previousHideMenus := config.HideMenus
	config.HideMenus = false
	_, err := syncDB.FindSatisfier(target.DepString())
	config.HideMenus = previousHideMenus
	if err == nil {
		return true
	}

	return !syncDB.FindGroupPkgs(target.Name).Empty()
}

func earlyPacmanCall(config *runtime.Configuration, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, args *types.Arguments) error {
	installArgs := args.Copy()
	installArgs.Op = "S"
	targets := args.Targets
	args.ClearTargets()
	installArgs.ClearTargets()

	syncDB, err := alpmHandle.SyncDBs()
	if err != nil {
		return err
	}

	if config.Mode.IsRepo() {
		installArgs.Targets = targets
	} else {
		//separate aur and repo targets
		for _, target := range targets {
			if inRepos(config, syncDB, target) {
				installArgs.AddTarget(target)
			} else {
				args.AddTarget(target)
			}
		}
	}

	if args.ExistsArg("y", "refresh") || args.ExistsArg("u", "sysupgrade") || len(installArgs.Targets) > 0 {
		err = exec.Show(exec.PassToPacman(config, pacmanConf, installArgs, config.NoConfirm))
		if err != nil {
			return fmt.Errorf("Error installing repo packages")
		}
	}

	return nil
}

func earlyRefresh(config *runtime.Configuration, pacmanConf *pacmanconf.Config, args *types.Arguments) error {
	refreshArgs := args.Copy()
	args.DelArg("y", "refresh")
	refreshArgs.DelArg("u", "sysupgrade")
	refreshArgs.DelArg("s", "search")
	refreshArgs.DelArg("i", "info")
	refreshArgs.DelArg("l", "list")
	refreshArgs.ClearTargets()
	return exec.Show(exec.PassToPacman(config, pacmanConf, refreshArgs, config.NoConfirm))
}

func asdeps(config *runtime.Configuration, pacmanConf *pacmanconf.Config, args *types.Arguments, pkgs []string, noconfirm bool) error {
	if len(pkgs) == 0 {
		return nil
	}

	depsArgs := args.CopyGlobal()
	depsArgs.AddArg("D", "asdeps")
	depsArgs.AddTarget(pkgs...)
	_, stderr, err := exec.Capture(exec.PassToPacman(config, pacmanConf, depsArgs, noconfirm))
	if err != nil {
		return fmt.Errorf("%s%s", stderr, err)
	}

	return nil
}

func asexp(config *runtime.Configuration, pacmanConf *pacmanconf.Config, args *types.Arguments, pkgs []string, noconfirm bool) error {
	if len(pkgs) == 0 {
		return nil
	}

	expArgs := args.CopyGlobal()
	expArgs.AddArg("D", "asexplicit")
	expArgs.AddTarget(pkgs...)
	_, stderr, err := exec.Capture(exec.PassToPacman(config, pacmanConf, expArgs, noconfirm))
	if err != nil {
		return fmt.Errorf("%s%s", stderr, err)
	}

	return nil
}

func cleanAfter(config *runtime.Configuration, bases []types.Base) {
	fmt.Println("removing Untracked AUR files from cache...")

	for i, base := range bases {
		dir := filepath.Join(config.BuildDir, base.Pkgbase())

		if exec.ShouldUseGit(dir, config.GitClone) {
			fmt.Printf(text.Bold(text.Cyan("::")+" Cleaning (%d/%d): %s\n"), i+1, len(bases), text.Cyan(dir))
			_, stderr, err := exec.Capture(exec.PassToGit(config.GitBin, config.GitFlags, dir, "reset", "--hard", "HEAD"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "error resetting %s: %s", base.String(), stderr)
			}

			exec.Show(exec.PassToGit(config.GitBin, config.GitFlags, dir, "clean", "-fx"))
		} else {
			fmt.Printf(text.Bold(text.Cyan("::")+" Deleting (%d/%d): %s\n"), i+1, len(bases), text.Cyan(dir))
			if err := os.RemoveAll(dir); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func removeMake(config *runtime.Configuration, pacmanConf *pacmanconf.Config, do *dep.Order, err *error) {
	removeArguments := types.MakeArguments()
	removeArguments.AddArg("R", "u")

	for _, pkg := range do.GetMake() {
		removeArguments.AddTarget(pkg)
	}

	*err = exec.Show(exec.PassToPacman(config, pacmanConf, removeArguments, true))
}

func anyExistInCache(bases []types.Base, dir string) bool {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(dir, pkg)

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func cleanBuilds(bases []types.Base, dir string) {
	for i, base := range bases {
		dir := filepath.Join(dir, base.Pkgbase())
		fmt.Printf(text.Bold(text.Cyan("::")+" Deleting (%d/%d): %s\n"), i+1, len(bases), text.Cyan(dir))
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func getIncompatible(alpmHandle *alpm.Handle, bases []types.Base, srcinfos map[string]*gosrc.Srcinfo, noConfirm bool) (types.StringSet, error) {
	incompatible := make(types.StringSet)
	basesMap := make(map[string]types.Base)
	alpmArch, err := alpmHandle.Arch()
	if err != nil {
		return nil, err
	}

nextpkg:
	for _, base := range bases {
		for _, arch := range srcinfos[base.Pkgbase()].Arch {
			if arch == "any" || arch == alpmArch {
				continue nextpkg
			}
		}

		incompatible.Set(base.Pkgbase())
		basesMap[base.Pkgbase()] = base
	}

	if len(incompatible) > 0 {
		fmt.Println()
		fmt.Print(text.Bold(text.Yellow(arrow)) + " The following packages are not compatible with your architecture:")
		for pkg := range incompatible {
			fmt.Print("  " + text.Cyan(basesMap[pkg].String()))
		}

		fmt.Println()

		if !text.ContinueTask("Try to build them anyway?", true, noConfirm) {
			return nil, fmt.Errorf("Aborting due to user")
		}
	}

	return incompatible, nil
}

func gitHasDiff(bin string, flags string, path string, name string) (bool, error) {
	stdout, stderr, err := exec.Capture(exec.PassToGit(bin, flags, filepath.Join(path, name), "rev-parse", "HEAD", "HEAD@{upstream}"))
	if err != nil {
		return false, fmt.Errorf("%s%s", stderr, err)
	}

	lines := strings.Split(stdout, "\n")
	head := lines[0]
	upstream := lines[1]

	return head != upstream, nil
}

func showPkgbuildDiffs(config *runtime.Configuration, bases []types.Base, cloned types.StringSet) error {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		if exec.ShouldUseGit(dir, config.GitClone) {
			start := "HEAD"

			if cloned.Get(pkg) {
				start = gitEmptyTree
			} else {
				hasDiff, err := gitHasDiff(config.GitBin, config.GitFlags, config.BuildDir, pkg)
				if err != nil {
					return err
				}

				if !hasDiff {
					fmt.Printf("%s %s: %s\n", text.Bold(text.Yellow(arrow)), text.Cyan(base.String()), text.Bold("No changes -- skipping"))
					continue
				}
			}

			args := []string{"diff", start + "..HEAD@{upstream}", "--src-prefix", dir + "/", "--dst-prefix", dir + "/", "--", ".", ":(exclude).SRCINFO"}
			if text.UseColor {
				args = append(args, "--color=always")
			} else {
				args = append(args, "--color=never")
			}
			err := exec.Show(exec.PassToGit(config.GitBin, config.GitFlags, dir, args...))
			if err != nil {
				return err
			}
		} else {
			args := []string{"diff"}
			if text.UseColor {
				args = append(args, "--color=always")
			} else {
				args = append(args, "--color=never")
			}
			args = append(args, "--no-index", "/var/empty", dir)
			// git always returns 1. why? I have no idea
			exec.Show(exec.PassToGit(config.GitBin, config.GitFlags, dir, args...))
		}
	}

	return nil
}

func gitMerge(bin string, flags, path string, name string) error {
	_, stderr, err := exec.Capture(exec.PassToGit(bin, flags, filepath.Join(path, name), "reset", "--hard", "HEAD"))
	if err != nil {
		return fmt.Errorf("error resetting %s: %s", name, stderr)
	}

	_, stderr, err = exec.Capture(exec.PassToGit(bin, flags, filepath.Join(path, name), "merge", "--no-edit", "--ff"))
	if err != nil {
		return fmt.Errorf("error merging %s: %s", name, stderr)
	}

	return nil
}
func mergePkgbuilds(config *runtime.Configuration, bases []types.Base) error {
	for _, base := range bases {
		if exec.ShouldUseGit(filepath.Join(config.BuildDir, base.Pkgbase()), config.GitClone) {
			err := gitMerge(config.GitBin, config.GitFlags, config.BuildDir, base.Pkgbase())
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func editPkgbuilds(config *runtime.Configuration, bases []types.Base, srcinfos map[string]*gosrc.Srcinfo) error {
	pkgbuilds := make([]string, 0, len(bases))
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		pkgbuilds = append(pkgbuilds, filepath.Join(dir, "PKGBUILD"))

		for _, splitPkg := range srcinfos[pkg].SplitPackages() {
			if splitPkg.Install != "" {
				pkgbuilds = append(pkgbuilds, filepath.Join(dir, splitPkg.Install))
			}
		}
	}

	if len(pkgbuilds) > 0 {
		editor, editorArgs := exec.Editor(config.Editor, config.EditorFlags, config.NoConfirm)
		if err := exec.ShowBin(editor, append(editorArgs, pkgbuilds...)...); err != nil { // To fix ungodly mess. Refactor Editor
			return fmt.Errorf("Editor did not exit successfully, Aborting: %s", err)
		}
	}

	return nil
}

// Move to download
func downloadPkgbuildsSources(config *runtime.Configuration, bases []types.Base, incompatible types.StringSet) (err error) {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		args := []string{"--verifysource", "-Ccf"}

		if incompatible.Get(pkg) {
			args = append(args, "--ignorearch")
		}

		err = exec.Show(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, args...))
		if err != nil {
			return fmt.Errorf("Error downloading sources: %s", text.Cyan(base.String()))
		}
	}

	return
}

func buildInstallPkgbuilds(config *runtime.Configuration,
	pacmanConf *pacmanconf.Config,
	alpmHandle *alpm.Handle,
	dp *dep.Pool, do *dep.Order,
	srcinfos map[string]*gosrc.Srcinfo,
	args *types.Arguments,
	incompatible types.StringSet, conflicts types.MapStringSet,
	savedInfo vcs.InfoStore) error {

	arguments := args.Copy()
	arguments.ClearTargets()
	arguments.Op = "U"
	arguments.DelArg("confirm")
	arguments.DelArg("noconfirm")
	arguments.DelArg("c", "clean")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("q", "quiet")
	arguments.DelArg("y", "refresh")
	arguments.DelArg("u", "sysupgrade")
	arguments.DelArg("w", "downloadonly")

	deps := make([]string, 0)
	exp := make([]string, 0)

	doInstall := func() error {
		if len(arguments.Targets) == 0 {
			return nil
		}

		err := exec.Show(exec.PassToPacman(config, pacmanConf, arguments, true))
		if err != nil {
			return err
		}

		err = savedInfo.Save() // Change stutter
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		if err = asdeps(config, pacmanConf, args, deps, true); err != nil {
			return err
		}
		if err = asexp(config, pacmanConf, args, exp, true); err != nil {
			return err
		}

		arguments.ClearTargets()
		deps = make([]string, 0)
		exp = make([]string, 0)
		config.NoConfirm = true
		return nil
	}

	for _, base := range do.Aur {
		var err error
		pkg := base.Pkgbase()
		dir := filepath.Join(config.BuildDir, pkg)
		built := true

		satisfied := true
	all:
		for _, pkg := range base {
			for _, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
				for _, dep := range deps {
					if _, err := dp.LocalDB.PkgCache().FindSatisfier(dep); err != nil {
						satisfied = false
						fmt.Printf("%s not satisfied, flushing install queue", dep)
						break all
					}
				}
			}
		}

		if !satisfied || !config.BatchInstall {
			err = doInstall()
			if err != nil {
				return err
			}
		}

		srcinfo := srcinfos[pkg]

		makepkgArgs := []string{"--nobuild", "-fC"}

		if incompatible.Get(pkg) {
			makepkgArgs = append(makepkgArgs, "--ignorearch")
		}

		//pkgver bump
		err = exec.Show(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, makepkgArgs...))
		if err != nil {
			return fmt.Errorf("Error making: %s", base.String())
		}

		pkgdests, version, err := parsePackageList(config, dir)
		if err != nil {
			return err
		}

		isExplicit := false
		for _, b := range base {
			isExplicit = isExplicit || dp.Explicit.Get(b.Name)
		}
		if config.ReBuild == "no" || (config.ReBuild == "yes" && !isExplicit) {
			for _, split := range base {
				pkgdest, ok := pkgdests[split.Name]
				if !ok {
					return fmt.Errorf("Could not find PKGDEST for: %s", split.Name)
				}

				_, err := os.Stat(pkgdest)
				if os.IsNotExist(err) {
					built = false
				} else if err != nil {
					return err
				}
			}
		} else {
			built = false
		}

		if args.ExistsArg("needed") {
			installed := true
			for _, split := range base {
				if alpmpkg := dp.LocalDB.Pkg(split.Name); alpmpkg == nil || alpmpkg.Version() != version {
					installed = false
				}
			}

			if installed {
				exec.Show(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
				fmt.Println(text.Cyan(pkg+"-"+version) + text.Bold(" is up to date -- skipping"))
				continue
			}
		}

		if built {
			exec.Show(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, "-c", "--nobuild", "--noextract", "--ignorearch"))
			fmt.Println(text.Bold(text.Yellow(arrow)),
				text.Cyan(pkg+"-"+version)+text.Bold(" already made -- skipping build"))
		} else {
			args := []string{"-cf", "--noconfirm", "--noextract", "--noprepare", "--holdver"}

			if incompatible.Get(pkg) {
				args = append(args, "--ignorearch")
			}

			err := exec.Show(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, args...))
			if err != nil {
				return fmt.Errorf("Error making: %s", base.String())
			}
		}

		//conflicts have been checked so answer y for them
		if config.UseAsk {
			ask, _ := strconv.Atoi(args.Globals["ask"])
			uask := alpm.QuestionType(ask) | alpm.QuestionTypeConflictPkg
			args.Globals["ask"] = fmt.Sprint(uask)
		} else {
			for _, split := range base {
				if _, ok := conflicts[split.Name]; ok {
					config.NoConfirm = false
					break
				}
			}
		}

		//remotenames: names of all non repo packages on the system
		_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
		if err != nil {
			return err
		}

		//cache as a stringset. maybe make it return a string set in the first
		//place
		remoteNamesCache := types.SliceToStringSet(remoteNames)
		localNamesCache := types.SliceToStringSet(localNames)

		for _, split := range base {
			pkgdest, ok := pkgdests[split.Name]
			if !ok {
				return fmt.Errorf("Could not find PKGDEST for: %s", split.Name)
			}

			arguments.AddTarget(pkgdest)
			if args.ExistsArg("asdeps", "asdep") {
				deps = append(deps, split.Name)
			} else if args.ExistsArg("asexplicit", "asexp") {
				exp = append(exp, split.Name)
			} else if !dp.Explicit.Get(split.Name) && !localNamesCache.Get(split.Name) && !remoteNamesCache.Get(split.Name) {
				deps = append(deps, split.Name)
			}
		}

		var mux sync.Mutex
		var wg sync.WaitGroup
		for _, pkg := range base {
			wg.Add(1)
			go savedInfo.Update(config, pkg.Name, srcinfo.Source, &mux, &wg) // Remove stutter
		}

		wg.Wait()
	}

	err := doInstall()
	return err
}

func parsePackageList(config *runtime.Configuration, dir string) (map[string]string, string, error) {
	stdout, stderr, err := exec.Capture(exec.PassToMakepkg(config.MakepkgBin, config.MFlags, config.MakepkgConf, dir, "--packagelist"))

	if err != nil {
		return nil, "", fmt.Errorf("%s%s", stderr, err)
	}

	var version string
	lines := strings.Split(stdout, "\n")
	pkgdests := make(map[string]string)

	for _, line := range lines {
		if line == "" {
			continue
		}

		fileName := filepath.Base(line)
		split := strings.Split(fileName, "-")

		if len(split) < 4 {
			return nil, "", fmt.Errorf("Can not find package name : %s", split)
		}

		// pkgname-pkgver-pkgrel-arch.pkgext
		// This assumes 3 dashes after the pkgname, Will cause an error
		// if the PKGEXT contains a dash. Please no one do that.
		pkgname := strings.Join(split[:len(split)-3], "-")
		version = strings.Join(split[len(split)-3:len(split)-1], "-")
		pkgdests[pkgname] = line
	}

	return pkgdests, version, nil
}

func Install(config *runtime.Configuration, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, args *types.Arguments, savedInfo vcs.InfoStore) (err error) {
	var incompatible types.StringSet
	var do *dep.Order

	var aurUp upgrade.UpSlice
	var repoUp upgrade.UpSlice

	var srcinfos map[string]*gosrc.Srcinfo

	warnings := &types.AURWarnings{}

	if config.Mode.IsAnyOrRepo() {
		if config.CombinedUpgrade {
			if args.ExistsArg("y", "refresh") {
				err = earlyRefresh(config, pacmanConf, args)
				if err != nil {
					return fmt.Errorf("Error refreshing databases")
				}
			}
		} else if args.ExistsArg("y", "refresh") || args.ExistsArg("u", "sysupgrade") || len(args.Targets) > 0 {
			err = earlyPacmanCall(config, pacmanConf, alpmHandle, args)
			if err != nil {
				return err
			}
		}
	}

	//we may have done -Sy, our handle now has an old
	//database.
	alpmHandle, err = runtime.InitAlpmHandle(config, pacmanConf, alpmHandle)
	if err != nil {
		return err
	}

	_, _, localNames, remoteNames, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return err
	}

	remoteNamesCache := types.SliceToStringSet(remoteNames)
	localNamesCache := types.SliceToStringSet(localNames)

	requestTargets := args.Copy().Targets

	//create the arguments to pass for the repo install
	arguments := args.Copy()
	arguments.DelArg("asdeps", "asdep")
	arguments.DelArg("asexplicit", "asexp")
	arguments.Op = "S"
	arguments.ClearTargets()

	if config.Mode.IsAUR() {
		arguments.DelArg("u", "sysupgrade")
	}

	//if we are doing -u also request all packages needing update
	if args.ExistsArg("u", "sysupgrade") {
		aurUp, repoUp, err = upgrade.UpList(config, alpmHandle, args, savedInfo, warnings)
		if err != nil {
			return err
		}

		warnings.Print()

		ignore, aurUp, err := upgradePkgs(config, alpmHandle, aurUp, repoUp)
		if err != nil {
			return err
		}

		for _, up := range repoUp {
			if !ignore.Get(up.Name) {
				requestTargets = append(requestTargets, up.Name)
				args.AddTarget(up.Name)
			}
		}

		for up := range aurUp {
			requestTargets = append(requestTargets, "aur/"+up)
			args.AddTarget("aur/" + up)
		}

		value, _, exists := args.GetArg("ignore")

		if len(ignore) > 0 {
			ignoreStr := strings.Join(ignore.ToSlice(), ",")
			if exists {
				ignoreStr += "," + value
			}
			arguments.Options["ignore"] = ignoreStr
		}
	}

	targets := types.SliceToStringSet(args.Targets)

	dp, err := dep.GetPool(config, args, alpmHandle, requestTargets, warnings)
	if err != nil {
		return err
	}

	err = dp.CheckMissing()
	if err != nil {
		return err
	}

	if len(dp.Aur) == 0 {
		if !config.CombinedUpgrade {
			if args.ExistsArg("u", "sysupgrade") {
				fmt.Println(" there is nothing to do")
			}
			return nil
		}

		args.Op = "S"
		args.DelArg("y", "refresh")
		args.Options["ignore"] = arguments.Options["ignore"]
		return exec.Show(exec.PassToPacman(config, pacmanConf, args, config.NoConfirm))
	}

	if len(dp.Aur) > 0 && os.Geteuid() == 0 {
		return fmt.Errorf(text.Bold(text.Red(arrow)) + " Refusing to install AUR Packages as root, Aborting.")
	}

	conflicts, err := dp.CheckConflicts(config.UseAsk, config.NoConfirm)
	if err != nil {
		return err
	}

	do = dep.GetOrder(dp)
	if err != nil {
		return err
	}

	for _, pkg := range do.Repo {
		arguments.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
	}

	for _, pkg := range dp.Groups {
		arguments.AddTarget(pkg)
	}

	if len(do.Aur) == 0 && len(arguments.Targets) == 0 && (!args.ExistsArg("u", "sysupgrade") || config.Mode.IsAUR()) {
		fmt.Println(" there is nothing to do")
		return nil
	}

	do.Print()
	fmt.Println()

	if config.CleanAfter {
		defer cleanAfter(config, do.Aur)
	}

	if do.HasMake() {
		switch config.RemoveMake {
		case "yes":
			defer removeMake(config, pacmanConf, do, &err)
		case "no":
			break
		default:
			if text.ContinueTask("Remove make dependencies after install?", false, config.NoConfirm) {
				defer removeMake(config, pacmanConf, do, &err)
			}
		}
	}

	if config.CleanMenu {
		if anyExistInCache(do.Aur, config.BuildDir) {
			askClean := pkgbuildNumberMenu(do.Aur, remoteNamesCache, config.BuildDir)
			toClean, err := cleanNumberMenu(config, do.Aur, remoteNamesCache, askClean)
			if err != nil {
				return err
			}

			cleanBuilds(toClean, config.BuildDir)
		}
	}

	toSkip := pkgbuildsToSkip(config, do.Aur, targets)
	cloned, err := download.Pkgbuilds(config, do.Aur, toSkip, config.BuildDir)
	if err != nil {
		return err
	}

	var toDiff []types.Base
	var toEdit []types.Base

	if config.DiffMenu {
		pkgbuildNumberMenu(do.Aur, remoteNamesCache, config.BuildDir)
		toDiff, err = diffNumberMenu(config, do.Aur, remoteNamesCache)
		if err != nil {
			return err
		}

		if len(toDiff) > 0 {
			err = showPkgbuildDiffs(config, toDiff, cloned)
			if err != nil {
				return err
			}
		}
	}

	if len(toDiff) > 0 {
		oldValue := config.NoConfirm
		config.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(text.Bold(text.Green("Proceed with install?")), true, config.NoConfirm) {
			return fmt.Errorf("Aborting due to user")
		}
		config.NoConfirm = oldValue
	}

	err = mergePkgbuilds(config, do.Aur)
	if err != nil {
		return err
	}

	srcinfos, err = parseSrcinfoFiles(do.Aur, config.BuildDir, true)
	if err != nil {
		return err
	}

	if config.EditMenu {
		pkgbuildNumberMenu(do.Aur, remoteNamesCache, config.BuildDir)
		toEdit, err = editNumberMenu(config, do.Aur, remoteNamesCache)
		if err != nil {
			return err
		}

		if len(toEdit) > 0 {
			err = editPkgbuilds(config, toEdit, srcinfos)
			if err != nil {
				return err
			}
		}
	}

	if len(toEdit) > 0 {
		oldValue := config.NoConfirm
		config.NoConfirm = false
		fmt.Println()
		if !text.ContinueTask(text.Bold(text.Green("Proceed with install?")), true, config.NoConfirm) {
			return fmt.Errorf("Aborting due to user")
		}
		config.NoConfirm = oldValue
	}

	incompatible, err = getIncompatible(alpmHandle, do.Aur, srcinfos, config.NoConfirm)
	if err != nil {
		return err
	}

	if config.PGPFetch {
		err = pgp.CheckKeys(config.GpgBin, config.GpgFlags, do.Aur, srcinfos, config.NoConfirm)
		if err != nil {
			return err
		}
	}

	if !config.CombinedUpgrade {
		arguments.DelArg("u", "sysupgrade")
	}

	if len(arguments.Targets) > 0 || arguments.ExistsArg("u") {
		err := exec.Show(exec.PassToPacman(config, pacmanConf, arguments, config.NoConfirm))
		if err != nil {
			return fmt.Errorf("Error installing repo packages")
		}

		deps := make([]string, 0)
		exp := make([]string, 0)

		for _, pkg := range dp.Repo {
			if !dp.Explicit.Get(pkg.Name()) && !localNamesCache.Get(pkg.Name()) && !remoteNamesCache.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
				continue
			}

			if args.ExistsArg("asdeps", "asdep") && dp.Explicit.Get(pkg.Name()) {
				deps = append(deps, pkg.Name())
			} else if args.ExistsArg("asexp", "asexplicit") && dp.Explicit.Get(pkg.Name()) {
				exp = append(exp, pkg.Name())
			}
		}

		if err = asdeps(config, pacmanConf, args, deps, config.NoConfirm); err != nil {
			return err
		}
		if err = asexp(config, pacmanConf, args, exp, config.NoConfirm); err != nil {
			return err
		}
	}

	go completion.Update(alpmHandle, config.AURURL, config.BuildDir, config.CompletionInterval, false)

	err = downloadPkgbuildsSources(config, do.Aur, incompatible)
	if err != nil {
		return err
	}

	err = buildInstallPkgbuilds(config, pacmanConf, alpmHandle, dp, do, srcinfos, args, incompatible, conflicts, savedInfo)
	if err != nil {
		return err
	}

	return nil
}
