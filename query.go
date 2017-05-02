package main

import (
	"fmt"

	"github.com/jguer/yay/aur"
	pac "github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
	rpc "github.com/mikkeloscar/aur"
)

// PrintSearch handles printing search results in a given format
func printAURSearch(q aur.Query, start int) {
	h, err := util.Conf.CreateHandle()
	defer h.Release()
	if err != nil {
	}

	localDb, _ := h.LocalDb()

	for i, res := range q {
		var toprint string
		if util.SearchVerbosity == util.NumberMenu {
			if util.SortMode == util.BottomUp {
				toprint += fmt.Sprintf("%d ", len(q)+start-i-1)
			} else {
				toprint += fmt.Sprintf("%d ", start+i)
			}
		} else if util.SearchVerbosity == util.Minimal {
			fmt.Println(res.Name)
			continue
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m(%d) ", "aur", res.Name, res.Version, res.NumVotes)
		if res.Maintainer == "" {
			toprint += fmt.Sprintf("\x1b[31;40m(Orphaned)\x1b[0m ")
		}

		if res.OutOfDate != 0 {
			toprint += fmt.Sprintf("\x1b[31;40m(Out-of-date)\x1b[0m ")
		}

		if _, err := localDb.PkgByName(res.Name); err == nil {
			toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
		}
		toprint += "\n" + res.Description
		fmt.Println(toprint)
	}

	return
}

// SyncSearch presents a query to the local repos and to the AUR.
func syncSearch(pkgS []string) (err error) {
	aq, err := aur.NarrowSearch(pkgS, true)
	if err != nil {
		return err
	}
	pq, _, err := pac.Search(pkgS)
	if err != nil {
		return err
	}

	if util.SortMode == util.BottomUp {
		printAURSearch(aq, 0)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		printAURSearch(aq, 0)
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func syncInfo(pkgS []string, flags []string) (err error) {
	aurS, repoS, err := pac.PackageSlices(pkgS)
	if err != nil {
		return
	}

	q, err := rpc.Info(aurS)
	if err != nil {
		fmt.Println(err)
	}

	for _, aurP := range q {
		aur.AURPrintInfo(&aurP)
	}

	if len(repoS) != 0 {
		err = passToPacman("-Si", repoS, flags)
	}

	return
}

// LocalStatistics returns installed packages statistics.
func localStatistics(version string) error {
	info, err := pac.Statistics()
	if err != nil {
		return err
	}

	foreignS, foreign, _ := pac.ForeignPackages()

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", info.Totaln)
	fmt.Printf("\x1B[1;32mTotal foreign installed packages: \x1B[0;33m%d\x1B[0m\n", foreign)
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", info.Expln)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", size(info.TotalSize))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	pac.BiggestPackages()
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")

	keys := make([]string, len(foreignS))
	i := 0
	for k := range foreignS {
		keys[i] = k
		i++
	}
	q, err := rpc.Info(keys)
	if err != nil {
		return err
	}

	for _, res := range q {
		if res.Maintainer == "" {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is orphaned.\x1b[0m\n", res.Name)
		}
		if res.OutOfDate != 0 {
			fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is out-of-date in AUR.\x1b[0m\n", res.Name)
		}
	}

	return nil
}
