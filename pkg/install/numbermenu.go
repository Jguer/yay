package install

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Jguer/yay/v10/pkg/vcs"
	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

const smallArrow = " ->"
const arrow = "==>"

func pkgbuildNumberMenu(bases []types.Base, installed types.StringSet, dir string) bool {
	toPrint := ""
	askClean := false

	for n, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(dir, pkg)

		toPrint += fmt.Sprintf(text.Magenta("%3d")+" %-40s", len(bases)-n,
			text.Bold(base.String()))

		anyInstalled := false
		for _, b := range base {
			anyInstalled = anyInstalled || installed.Get(b.Name)
		}

		if anyInstalled {
			toPrint += text.Bold(text.Green(" (Installed)"))
		}

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			toPrint += text.Bold(text.Green(" (Build Files Exist)"))
			askClean = true
		}

		toPrint += "\n"
	}

	fmt.Print(toPrint)

	return askClean
}

func cleanNumberMenu(config *runtime.Configuration, bases []types.Base, installed types.StringSet, hasClean bool) ([]types.Base, error) {
	toClean := make([]types.Base, 0)

	if !hasClean {
		return toClean, nil
	}

	fmt.Println(text.Bold(text.Green(arrow + " Packages to cleanBuild?")))
	fmt.Println(text.Bold(text.Green(arrow) + text.Cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))
	fmt.Print(text.Bold(text.Green(arrow + " ")))
	cleanInput, err := text.GetInput(config.AnswerClean, config.NoConfirm)
	if err != nil {
		return nil, err
	}

	cInclude, cExclude, cOtherInclude, cOtherExclude := types.ParseNumberMenu(cleanInput)
	cIsInclude := len(cExclude) == 0 && len(cOtherExclude) == 0

	if cOtherInclude.Get("abort") || cOtherInclude.Get("ab") {
		return nil, fmt.Errorf("Aborting due to user")
	}

	if !cOtherInclude.Get("n") && !cOtherInclude.Get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.Get(b.Name)
			}

			dir := filepath.Join(config.BuildDir, pkg)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue
			}

			if !cIsInclude && cExclude.Get(len(bases)-i) {
				continue
			}

			if anyInstalled && (cOtherInclude.Get("i") || cOtherInclude.Get("installed")) {
				toClean = append(toClean, base)
				continue
			}

			if !anyInstalled && (cOtherInclude.Get("no") || cOtherInclude.Get("notinstalled")) {
				toClean = append(toClean, base)
				continue
			}

			if cOtherInclude.Get("a") || cOtherInclude.Get("all") {
				toClean = append(toClean, base)
				continue
			}

			if cIsInclude && (cInclude.Get(len(bases)-i) || cOtherInclude.Get(pkg)) {
				toClean = append(toClean, base)
				continue
			}

			if !cIsInclude && (!cExclude.Get(len(bases)-i) && !cOtherExclude.Get(pkg)) {
				toClean = append(toClean, base)
				continue
			}
		}
	}

	return toClean, nil
}

func editNumberMenu(config *runtime.Configuration, bases []types.Base, installed types.StringSet) ([]types.Base, error) {
	return editDiffNumberMenu(config, bases, installed, false)
}

func diffNumberMenu(config *runtime.Configuration, bases []types.Base, installed types.StringSet) ([]types.Base, error) {
	return editDiffNumberMenu(config, bases, installed, true)
}

func editDiffNumberMenu(config *runtime.Configuration, bases []types.Base, installed types.StringSet, diff bool) ([]types.Base, error) {
	toEdit := make([]types.Base, 0)
	var editInput string
	var err error

	if diff {
		fmt.Println(text.Bold(text.Green(arrow + " Diffs to show?")))
		fmt.Println(text.Bold(text.Green(arrow) + text.Cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))
		fmt.Print(text.Bold(text.Green(arrow + " ")))
		editInput, err = text.GetInput(config.AnswerDiff, config.NoConfirm)
		if err != nil {
			return nil, err
		}
	} else {
		fmt.Println(text.Bold(text.Green(arrow + " PKGBUILDs to edit?")))
		fmt.Println(text.Bold(text.Green(arrow) + text.Cyan(" [N]one ") + "[A]ll [Ab]ort [I]nstalled [No]tInstalled or (1 2 3, 1-3, ^4)"))
		fmt.Print(text.Bold(text.Green(arrow + " ")))
		editInput, err = text.GetInput(config.AnswerEdit, config.NoConfirm)
		if err != nil {
			return nil, err
		}
	}

	eInclude, eExclude, eOtherInclude, eOtherExclude := types.ParseNumberMenu(editInput)
	eIsInclude := len(eExclude) == 0 && len(eOtherExclude) == 0

	if eOtherInclude.Get("abort") || eOtherInclude.Get("ab") {
		return nil, fmt.Errorf("Aborting due to user")
	}

	if !eOtherInclude.Get("n") && !eOtherInclude.Get("none") {
		for i, base := range bases {
			pkg := base.Pkgbase()
			anyInstalled := false
			for _, b := range base {
				anyInstalled = anyInstalled || installed.Get(b.Name)
			}

			if !eIsInclude && eExclude.Get(len(bases)-i) {
				continue
			}

			if anyInstalled && (eOtherInclude.Get("i") || eOtherInclude.Get("installed")) {
				toEdit = append(toEdit, base)
				continue
			}

			if !anyInstalled && (eOtherInclude.Get("no") || eOtherInclude.Get("notinstalled")) {
				toEdit = append(toEdit, base)
				continue
			}

			if eOtherInclude.Get("a") || eOtherInclude.Get("all") {
				toEdit = append(toEdit, base)
				continue
			}

			if eIsInclude && (eInclude.Get(len(bases)-i) || eOtherInclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}

			if !eIsInclude && (!eExclude.Get(len(bases)-i) && !eOtherExclude.Get(pkg)) {
				toEdit = append(toEdit, base)
			}
		}
	}

	return toEdit, nil
}

// DisplayNumberMenu presents a CLI for selecting packages to install.
func DisplayNumberMenu(config *runtime.Configuration, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, savedInfo vcs.InfoStore, cmdArgs *types.Arguments, pkgS []string) (err error) {
	var (
		aurErr, repoErr error
		aq              query.AUR
		pq              query.Repo
		lenaq, lenpq    int
	)

	pkgS = query.RemoveInvalidTargets(config.Mode, pkgS)

	if config.Mode.IsAnyOrAUR() {
		aq, aurErr = query.AURNarrow(pkgS, true, config.SortBy, config.SortMode)
		lenaq = len(aq)
	}
	if config.Mode.IsAnyOrRepo() {
		pq, repoErr = query.RepoSimple(pkgS, alpmHandle, config.SortMode)
		lenpq = len(pq)
		if repoErr != nil {
			return err
		}
	}

	if lenpq == 0 && lenaq == 0 {
		return fmt.Errorf("No packages match search")
	}

	switch config.SortMode {
	case runtime.TopDown:
		if config.Mode.IsAnyOrRepo() {
			pq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode)
		}
		if config.Mode.IsAnyOrAUR() {
			aq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode, lenpq+1)
		}
	case runtime.BottomUp:
		if config.Mode.IsAnyOrAUR() {
			aq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode, lenpq+1)
		}
		if config.Mode.IsAnyOrRepo() {
			pq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode)
		}
	default:
		return fmt.Errorf("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
	}

	if aurErr != nil {
		fmt.Fprintf(os.Stderr, "Error during AUR search: %s\n", aurErr)
		fmt.Fprintln(os.Stderr, "Showing repo packages only")
	}

	fmt.Println(text.Bold(text.Green(arrow + " Packages to install (eg: 1 2 3, 1-3 or ^4)")))
	fmt.Print(text.Bold(text.Green(arrow + " ")))

	reader := bufio.NewReader(os.Stdin)

	numberBuf, overflow, err := reader.ReadLine()
	if err != nil {
		return err
	}
	if overflow {
		return fmt.Errorf("Input too long")
	}

	include, exclude, _, otherExclude := types.ParseNumberMenu(string(numberBuf))
	arguments := cmdArgs.CopyGlobal()

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range pq {
		var target int
		switch config.SortMode {
		case runtime.TopDown:
			target = i + 1
		case runtime.BottomUp:
			target = len(pq) - i
		default:
			return fmt.Errorf("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.AddTarget(pkg.DB().Name() + "/" + pkg.Name())
		}
	}

	for i, pkg := range aq {
		var target int

		switch config.SortMode {
		case runtime.TopDown:
			target = i + 1 + len(pq)
		case runtime.BottomUp:
			target = len(aq) - i + len(pq)
		default:
			return fmt.Errorf("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
		}

		if (isInclude && include.Get(target)) || (!isInclude && !exclude.Get(target)) {
			arguments.AddTarget("aur/" + pkg.Name)
		}
	}

	if len(arguments.Targets) == 0 {
		fmt.Println("There is nothing to do")
		return nil
	}

	if config.SudoLoop {
		exec.SudoLoopBackground()
	}

	err = Install(config, pacmanConf, alpmHandle, arguments, savedInfo)

	return err
}
