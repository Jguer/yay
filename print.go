package main

import (
	"fmt"
	"strings"

	rpc "github.com/mikkeloscar/aur"
)

// Human returns results in Human readable format.
func human(size int64) string {
	floatsize := float32(size)
	units := [...]string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	for _, unit := range units {
		if floatsize < 1024 {
			return fmt.Sprintf("%.1f %sB", floatsize, unit)
		}
		floatsize /= 1024
	}
	return fmt.Sprintf("%d%s", size, "B")
}

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int) {
	localDb, _ := alpmHandle.LocalDb()

	for i, res := range q {
		var toprint string
		if config.SearchMode == NumberMenu {
			if config.SortMode == BottomUp {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", len(q)+start-i-1)
			} else {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", start+i)
			}
		} else if config.SearchMode == Minimal {
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
		toprint += "\n    " + res.Description
		fmt.Println(toprint)
	}
}

//PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch() {
	for i, res := range s {
		var toprint string
		if config.SearchMode == NumberMenu {
			if config.SortMode == BottomUp {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", len(s)-i)
			} else {
				toprint += fmt.Sprintf("\x1b[33m%d\x1b[0m ", i+1)
			}
		} else if config.SearchMode == Minimal {
			fmt.Println(res.Name())
			continue
		}
		toprint += fmt.Sprintf("\x1b[1m%s/\x1b[33m%s \x1b[36m%s \x1b[0m",
			res.DB().Name(), res.Name(), res.Version())

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := alpmHandle.LocalDb()
		if err == nil {
			if _, err = localDb.PkgByName(res.Name()); err == nil {
				toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

func printDeps(repoDeps []string, aurDeps []string) {
	if len(repoDeps) != 0 {
		fmt.Print("\x1b[1;32m==> Repository dependencies: \x1b[0m")
		for _, repoD := range repoDeps {
			fmt.Print("\x1b[33m", repoD, " \x1b[0m")
		}
		fmt.Print("\n")

	}
	if len(aurDeps) != 0 {
		fmt.Print("\x1b[1;32m==> AUR dependencies: \x1b[0m")
		for _, aurD := range aurDeps {
			fmt.Print("\x1b[33m", aurD, " \x1b[0m")
		}
		fmt.Print("\n")
	}
}

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg) {
	fmt.Println("\x1b[1;37mRepository      :\x1b[0m", "aur")
	fmt.Println("\x1b[1;37mName            :\x1b[0m", a.Name)
	fmt.Println("\x1b[1;37mVersion         :\x1b[0m", a.Version)
	fmt.Println("\x1b[1;37mDescription     :\x1b[0m", a.Description)
	if a.URL != "" {
		fmt.Println("\x1b[1;37mURL             :\x1b[0m", a.URL)
	} else {
		fmt.Println("\x1b[1;37mURL             :\x1b[0m", "None")
	}
	fmt.Println("\x1b[1;37mLicenses        :\x1b[0m", strings.Join(a.License, "  "))

	// if len(a.Provides) != 0 {
	// 	fmt.Println("\x1b[1;37mProvides        :\x1b[0m",
	// 	Strings.join(a.Provides, "  "))
	// } else {
	// 	fmt.Println("\x1b[1;37mProvides        :\x1b[0m", "None")
	// }

	if len(a.Depends) != 0 {
		fmt.Println("\x1b[1;37mDepends On      :\x1b[0m", strings.Join(a.Depends, "  "))
	} else {
		fmt.Println("\x1b[1;37mDepends On      :\x1b[0m", "None")
	}

	if len(a.MakeDepends) != 0 {
		fmt.Println("\x1b[1;37mMake depends On :\x1b[0m", strings.Join(a.MakeDepends, "  "))
	} else {
		fmt.Println("\x1b[1;37mMake depends On :\x1b[0m", "None")
	}

	if len(a.OptDepends) != 0 {
		fmt.Println("\x1b[1;37mOptional Deps   :\x1b[0m", strings.Join(a.OptDepends, "  "))
	} else {
		fmt.Println("\x1b[1;37mOptional Deps   :\x1b[0m", "None")
	}

	if len(a.Conflicts) != 0 {
		fmt.Println("\x1b[1;37mConflicts With  :\x1b[0m",strings.Join(a.Conflicts, "  "))
	} else {
		fmt.Println("\x1b[1;37mConflicts With  :\x1b[0m", "None")
	}

	if a.Maintainer != "" {
		fmt.Println("\x1b[1;37mMaintainer      :\x1b[0m", a.Maintainer)
	} else {
		fmt.Println("\x1b[1;37mMaintainer      :\x1b[0m", "None")
	}

	fmt.Println("\x1b[1;37mVotes           :\x1b[0m", a.NumVotes)
	fmt.Println("\x1b[1;37mPopularity      :\x1b[0m", a.Popularity)

	if a.OutOfDate != 0 {
		fmt.Println("\x1b[1;37mOut-of-date     :\x1b[0m", "Yes")
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages() {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return
	}

	pkgCache := localDb.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Printf("%s: \x1B[0;33m%s\x1B[0m\n", pkgS[i].Name(), human(pkgS[i].ISize()))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics() error {
	info, err := statistics()
	if err != nil {
		return err
	}

	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	fmt.Printf("\n Yay version r%s\n", version)
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Printf("\x1B[1;32mTotal installed packages: \x1B[0;33m%d\x1B[0m\n", info.Totaln)
	fmt.Printf("\x1B[1;32mTotal foreign installed packages: \x1B[0;33m%d\x1B[0m\n", len(remoteNames))
	fmt.Printf("\x1B[1;32mExplicitly installed packages: \x1B[0;33m%d\x1B[0m\n", info.Expln)
	fmt.Printf("\x1B[1;32mTotal Size occupied by packages: \x1B[0;33m%s\x1B[0m\n", human(info.TotalSize))
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")
	fmt.Println("\x1B[1;32mTen biggest packages\x1B[0m")
	biggestPackages()
	fmt.Println("\x1B[1;34m===========================================\x1B[0m")

	var q aurQuery
	var j int
	for i := len(remoteNames); i != 0; i = j {
		j = i - config.RequestSplitN
		if j < 0 {
			j = 0
		}
		qtemp, err := rpc.Info(remoteNames[j:i])
		q = append(q, qtemp...)
		if err != nil {
			return err
		}
	}

	var outcast []string
	for _, s := range remoteNames {
		found := false
		for _, i := range q {
			if s == i.Name {
				found = true
				break
			}
		}
		if !found {
			outcast = append(outcast, s)
		}
	}

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

	for _, res := range outcast {
		fmt.Printf("\x1b[1;31;40mWarning: \x1B[1;33;40m%s\x1b[0;37;40m is not available in AUR.\x1b[0m\n", res)
	}

	return nil
}
