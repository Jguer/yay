package main

import (
	"fmt"

	"github.com/jguer/yay/aur"
	pac "github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
)

// SyncSearch presents a query to the local repos and to the AUR.
func SyncSearch(pkgS []string) (err error) {
	aq, _, err := aur.Search(pkgS, true)
	if err != nil {
		return err
	}
	pq, _, err := pac.Search(pkgS)
	if err != nil {
		return err
	}

	if util.SortMode == util.BottomUp {
		aq.PrintSearch(0)
		pq.PrintSearch()
	} else {
		pq.PrintSearch()
		aq.PrintSearch(0)
	}

	return nil
}

// SyncInfo serves as a pacman -Si for repo packages and AUR packages.
func SyncInfo(pkgS []string, flags []string) (err error) {
	aurS, repoS, err := pac.PackageSlices(pkgS)
	if err != nil {
		return
	}

	q, _, err := aur.MultiInfo(aurS)
	if err != nil {
		fmt.Println(err)
	}

	for _, aurP := range q {
		aurP.PrintInfo()
	}

	if len(repoS) != 0 {
		err = PassToPacman("-Si", repoS, flags)
	}

	return
}

// LocalStatistics returns installed packages statistics.
func LocalStatistics(version string) error {
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
	q, _, err := aur.MultiInfo(keys)
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
