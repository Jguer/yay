package dep

import (
	"fmt"

	"github.com/Jguer/yay/v10/pkg/db"
	aur "github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

type Order struct {
	Aur     []Base
	Repo    []db.IPackage
	Runtime stringset.StringSet
}

func makeOrder() *Order {
	return &Order{
		make([]Base, 0),
		make([]db.IPackage, 0),
		make(stringset.StringSet),
	}
}

func GetOrder(dp *Pool, noDeps, noCheckDeps bool) *Order {
	do := makeOrder()

	for _, target := range dp.Targets {
		dep := target.DepString()

		if aurPkg := dp.Aur[dep]; aurPkg != nil && pkgSatisfies(aurPkg.Name, aurPkg.Version, dep) {
			do.orderPkgAur(aurPkg, dp, true, noDeps, noCheckDeps)
		} else if aurPkg := dp.findSatisfierAur(dep); aurPkg != nil {
			do.orderPkgAur(aurPkg, dp, true, noDeps, noCheckDeps)
		} else if repoPkg := dp.findSatisfierRepo(dep); repoPkg != nil {
			do.orderPkgRepo(repoPkg, dp, true)
		}
	}

	return do
}

func (do *Order) orderPkgAur(pkg *aur.Pkg, dp *Pool, runtime, noDeps, noCheckDeps bool) {
	if runtime {
		do.Runtime.Set(pkg.Name)
	}

	delete(dp.Aur, pkg.Name)

	for i, deps := range ComputeCombinedDepList(pkg, noDeps, noCheckDeps) {
		for _, dep := range deps {
			if aurPkg := dp.findSatisfierAur(dep); aurPkg != nil {
				do.orderPkgAur(aurPkg, dp, runtime && i == 0, noDeps, noCheckDeps)
			}

			if repoPkg := dp.findSatisfierRepo(dep); repoPkg != nil {
				do.orderPkgRepo(repoPkg, dp, runtime && i == 0)
			}
		}
	}

	for i, base := range do.Aur {
		if base.Pkgbase() == pkg.PackageBase {
			do.Aur[i] = append(base, pkg)
			return
		}
	}

	do.Aur = append(do.Aur, Base{pkg})
}

func (do *Order) orderPkgRepo(pkg db.IPackage, dp *Pool, runtime bool) {
	if runtime {
		do.Runtime.Set(pkg.Name())
	}

	delete(dp.Repo, pkg.Name())

	for _, dep := range dp.AlpmExecutor.PackageDepends(pkg) {
		if repoPkg := dp.findSatisfierRepo(dep.String()); repoPkg != nil {
			do.orderPkgRepo(repoPkg, dp, runtime)
		}
	}

	do.Repo = append(do.Repo, pkg)
}

func (do *Order) HasMake() bool {
	lenAur := 0
	for _, base := range do.Aur {
		lenAur += len(base)
	}

	return len(do.Runtime) != lenAur+len(do.Repo)
}

func (do *Order) GetMake() []string {
	makeOnly := []string{}

	for _, base := range do.Aur {
		for _, pkg := range base {
			if !do.Runtime.Get(pkg.Name) {
				makeOnly = append(makeOnly, pkg.Name)
			}
		}
	}

	for _, pkg := range do.Repo {
		if !do.Runtime.Get(pkg.Name()) {
			makeOnly = append(makeOnly, pkg.Name())
		}
	}

	return makeOnly
}

// Print prints repository packages to be downloaded.
func (do *Order) Print() {
	repo := ""
	repoMake := ""
	aurString := ""
	aurMake := ""

	repoLen := 0
	repoMakeLen := 0
	aurLen := 0
	aurMakeLen := 0

	for _, pkg := range do.Repo {
		pkgStr := fmt.Sprintf("  %s-%s", pkg.Name(), pkg.Version())
		if do.Runtime.Get(pkg.Name()) {
			repo += pkgStr
			repoLen++
		} else {
			repoMake += pkgStr
			repoMakeLen++
		}
	}

	for _, base := range do.Aur {
		pkg := base.Pkgbase()
		pkgStr := "  " + pkg + "-" + base[0].Version
		pkgStrMake := pkgStr

		push := false
		pushMake := false

		switch {
		case len(base) > 1, pkg != base[0].Name:
			pkgStr += " ("
			pkgStrMake += " ("

			for _, split := range base {
				if do.Runtime.Get(split.Name) {
					pkgStr += split.Name + " "
					aurLen++

					push = true
				} else {
					pkgStrMake += split.Name + " "
					aurMakeLen++
					pushMake = true
				}
			}

			pkgStr = pkgStr[:len(pkgStr)-1] + ")"
			pkgStrMake = pkgStrMake[:len(pkgStrMake)-1] + ")"
		case do.Runtime.Get(base[0].Name):
			aurLen++

			push = true
		default:
			aurMakeLen++

			pushMake = true
		}

		if push {
			aurString += pkgStr
		}

		if pushMake {
			aurMake += pkgStrMake
		}
	}

	printDownloads("Repo", repoLen, repo)
	printDownloads("Repo Make", repoMakeLen, repoMake)
	printDownloads("Aur", aurLen, aurString)
	printDownloads("Aur Make", aurMakeLen, aurMake)
}

func printDownloads(repoName string, length int, packages string) {
	if length < 1 {
		return
	}

	repoInfo := fmt.Sprintf(text.Bold(text.Blue("[%s:%d]")), repoName, length)
	fmt.Println(repoInfo + text.Cyan(packages))
}
