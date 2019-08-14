package dep

import (
	"fmt"
	"strconv"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	rpc "github.com/mikkeloscar/aur"
)

type Order struct {
	Aur     []types.Base
	Repo    []*alpm.Package
	Runtime types.StringSet
}

func MakeOrder() *Order {
	return &Order{
		make([]types.Base, 0),
		make([]*alpm.Package, 0),
		make(types.StringSet),
	}
}

func GetOrder(dp *Pool) *Order {
	do := MakeOrder()

	for _, target := range dp.Targets {
		dep := target.DepString()
		aurPkg := dp.Aur[dep]
		if aurPkg != nil && pkgSatisfies(aurPkg.Name, aurPkg.Version, dep) {
			do.OrderPkgAur(aurPkg, dp, true)
		}

		aurPkg = dp.findSatisfierAur(dep)
		if aurPkg != nil {
			do.OrderPkgAur(aurPkg, dp, true)
		}

		repoPkg := dp.findSatisfierRepo(dep)
		if repoPkg != nil {
			do.OrderPkgRepo(repoPkg, dp, true)
		}
	}

	return do
}

func (do *Order) OrderPkgAur(pkg *rpc.Pkg, dp *Pool, runtime bool) {
	if runtime {
		do.Runtime.Set(pkg.Name)
	}
	delete(dp.Aur, pkg.Name)

	for i, deps := range [3][]string{pkg.Depends, pkg.MakeDepends, pkg.CheckDepends} {
		for _, dep := range deps {
			aurPkg := dp.findSatisfierAur(dep)
			if aurPkg != nil {
				do.OrderPkgAur(aurPkg, dp, runtime && i == 0)
			}

			repoPkg := dp.findSatisfierRepo(dep)
			if repoPkg != nil {
				do.OrderPkgRepo(repoPkg, dp, runtime && i == 0)
			}
		}
	}

	for i, base := range do.Aur {
		if base.Pkgbase() == pkg.PackageBase {
			do.Aur[i] = append(base, pkg)
			return
		}
	}

	do.Aur = append(do.Aur, types.Base{pkg})
}

func (do *Order) OrderPkgRepo(pkg *alpm.Package, dp *Pool, runtime bool) {
	if runtime {
		do.Runtime.Set(pkg.Name())
	}
	delete(dp.Repo, pkg.Name())

	pkg.Depends().ForEach(func(dep alpm.Depend) (err error) {
		repoPkg := dp.findSatisfierRepo(dep.String())
		if repoPkg != nil {
			do.OrderPkgRepo(repoPkg, dp, runtime)
		}

		return nil
	})

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
	makeOnly := make([]string, 0, len(do.Aur)+len(do.Repo)-len(do.Runtime))

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

func printDownloads(repoName string, length int, packages string) {
	if length < 1 {
		return
	}

	repoInfo := text.Bold(text.Blue(
		"[" + repoName + ": " + strconv.Itoa(length) + "]"))
	fmt.Println(repoInfo + text.Cyan(packages))
}

// Print prints repository packages to be downloaded
func (do *Order) Print() {
	repo := ""
	repoMake := ""
	aur := ""
	aurMake := ""

	repoLen := 0
	repoMakeLen := 0
	aurLen := 0
	aurMakeLen := 0

	for _, pkg := range do.Repo {
		if do.Runtime.Get(pkg.Name()) {
			repo += "  " + pkg.Name() + "-" + pkg.Version()
			repoLen++
		} else {
			repoMake += "  " + pkg.Name() + "-" + pkg.Version()
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
			aur += pkgStr
		}
		if pushMake {
			aurMake += pkgStrMake
		}
	}

	printDownloads("Repo", repoLen, repo)
	printDownloads("Repo Make", repoMakeLen, repoMake)
	printDownloads("Aur", aurLen, aur)
	printDownloads("Aur Make", aurMakeLen, aurMake)
}
