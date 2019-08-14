package lookup

import (
	"bufio"
	"fmt"
	"net/http"
	"os"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/exec"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	"github.com/Morganamilo/go-pacmanconf"

	rpc "github.com/mikkeloscar/aur"
)

// PrintSearch presents a query to the local repos and to the AUR.
func PrintSearch(config *runtime.Configuration, alpmHandle *alpm.Handle, pkgS []string) (err error) {
	pkgS = query.RemoveInvalidTargets(config.Mode, pkgS)
	var aurErr error
	var repoErr error
	var aq query.AUR
	var pq query.Repo

	if config.Mode.IsAnyOrAUR() {
		aq, aurErr = query.AURNarrow(pkgS, true, config.SortBy, config.SortMode)
	}
	if config.Mode.IsAnyOrRepo() {
		pq, repoErr = query.RepoSimple(pkgS, alpmHandle, config.SortMode)
		if repoErr != nil {
			return err
		}
	}

	switch config.SortMode {
	case runtime.TopDown:
		if config.Mode.IsAnyOrRepo() {
			pq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode)
		}
		if config.Mode.IsAnyOrAUR() {
			aq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode, 1)
		}
	case runtime.BottomUp:
		if config.Mode.IsAnyOrAUR() {
			aq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode, 1)
		}
		if config.Mode.IsAnyOrRepo() {
			pq.PrintSearch(alpmHandle, config.SearchMode, config.SortMode)
		}
	default:
		return fmt.Errorf("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
	}

	if aurErr != nil {
		fmt.Fprintf(os.Stderr, "Error during AUR search: %s\n", aurErr)
		fmt.Fprintln(os.Stderr, "Showing Repo packages only")
	}

	return nil
}

// SyncList lists packages based on targets
func SyncList(config *runtime.Configuration, pacmanConf *pacmanconf.Config, alpmHandle *alpm.Handle, args *types.Arguments) error {
	aur := false

	for i := len(args.Targets) - 1; i >= 0; i-- {
		if args.Targets[i] == "aur" && (config.Mode.IsAnyOrAUR()) {
			args.Targets = append(args.Targets[:i], args.Targets[i+1:]...)
			aur = true
		}
	}

	if (config.Mode.IsAnyOrAUR()) && (len(args.Targets) == 0 || aur) {
		localDB, err := alpmHandle.LocalDB()
		if err != nil {
			return err
		}

		resp, err := http.Get(config.AURURL + "/packages.gz")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)

		scanner.Scan()
		for scanner.Scan() {
			name := scanner.Text()
			if args.ExistsArg("q", "quiet") {
				fmt.Println(name)
			} else {
				fmt.Printf("%s %s %s", text.Magenta("aur"), text.Bold(name), text.Bold(text.Green("unknown-version")))

				if localDB.Pkg(name) != nil {
					fmt.Print(text.Bold(text.Blue(" [Installed]")))
				}

				fmt.Println()
			}
		}
	}

	if (config.Mode.IsAnyOrRepo()) && (len(args.Targets) != 0 || !aur) {
		return exec.Show(exec.PassToPacman(config, pacmanConf, args, config.NoConfirm))
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func SyncInfo(config *runtime.Configuration, pacmanConf *pacmanconf.Config, cmdArgs *types.Arguments, alpmHandle *alpm.Handle, pkgS []string) (err error) {
	var info []*rpc.Pkg
	missing := false
	pkgS = query.RemoveInvalidTargets(config.Mode, pkgS)
	aurS, repoS, err := query.PackageSlices(alpmHandle, config.Mode, pkgS)
	if err != nil {
		return
	}

	if len(aurS) != 0 {
		noDB := make([]string, 0, len(aurS))

		for _, pkg := range aurS {
			_, name := query.SplitDBFromName(pkg)
			noDB = append(noDB, name)
		}

		info, err = query.AURInfoPrint(config, noDB)
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
		err = exec.Show(exec.PassToPacman(config, pacmanConf, arguments, config.NoConfirm))

		if err != nil {
			return
		}
	}

	if len(aurS) != len(info) {
		missing = true
	}

	if len(info) != 0 {
		for _, pkg := range info {
			printInfo(cmdArgs, config.AURURL, pkg)
		}
	}

	if missing {
		err = fmt.Errorf("")
	}

	return
}
