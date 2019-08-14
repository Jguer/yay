package dep

import (
	"fmt"
	"os"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

const (
	arrow      = "==>"
	smallArrow = " ->"
)

func (dp *Pool) checkInnerConflict(name string, conflict string, conflicts types.MapStringSet) {
	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			continue
		}

		if satisfiesAur(conflict, pkg) {
			conflicts.Add(name, pkg.Name)
		}
	}

	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			continue
		}

		if satisfiesRepo(conflict, pkg) {
			conflicts.Add(name, pkg.Name())
		}
	}
}

func (dp *Pool) checkForwardConflict(name string, conflict string, conflicts types.MapStringSet) {
	dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		if pkg.Name() == name || dp.hasPackage(pkg.Name()) {
			return nil
		}

		if satisfiesRepo(conflict, &pkg) {
			n := pkg.Name()
			if n != conflict {
				n += " (" + conflict + ")"
			}
			conflicts.Add(name, n)
		}

		return nil
	})
}

func (dp *Pool) checkReverseConflict(name string, conflict string, conflicts types.MapStringSet) {
	for _, pkg := range dp.Aur {
		if pkg.Name == name {
			continue
		}

		if satisfiesAur(conflict, pkg) {
			if name != conflict {
				name += " (" + conflict + ")"
			}

			conflicts.Add(pkg.Name, name)
		}
	}

	for _, pkg := range dp.Repo {
		if pkg.Name() == name {
			continue
		}

		if satisfiesRepo(conflict, pkg) {
			if name != conflict {
				name += " (" + conflict + ")"
			}

			conflicts.Add(pkg.Name(), name)
		}
	}
}

func (dp *Pool) checkInnerConflicts(conflicts types.MapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkInnerConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkInnerConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *Pool) checkForwardConflicts(conflicts types.MapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkForwardConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkForwardConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *Pool) checkReverseConflicts(conflicts types.MapStringSet) {
	dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		if dp.hasPackage(pkg.Name()) {
			return nil
		}

		pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkReverseConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})

		return nil
	})
}

// CheckConflicts checks packages in a pool for conflicts.
func (dp *Pool) CheckConflicts(ask bool, noconfirm bool) (types.MapStringSet, error) {
	var wg sync.WaitGroup
	innerConflicts := make(types.MapStringSet)
	conflicts := make(types.MapStringSet)
	wg.Add(2)

	fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Checking for conflicts...")))
	go func() {
		dp.checkForwardConflicts(conflicts)
		dp.checkReverseConflicts(conflicts)
		wg.Done()
	}()

	fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Checking for inner conflicts...")))
	go func() {
		dp.checkInnerConflicts(innerConflicts)
		wg.Done()
	}()

	wg.Wait()

	if len(innerConflicts) != 0 {
		fmt.Println()
		fmt.Println(text.Bold(text.Red(arrow)), text.Bold("Inner conflicts found:"))

		for name, pkgs := range innerConflicts {
			str := text.Red(text.Bold(smallArrow)) + " " + name + ":"
			for pkg := range pkgs {
				str += " " + text.Cyan(pkg) + ","
			}
			str = strings.TrimSuffix(str, ",")

			fmt.Println(str)
		}

	}

	if len(conflicts) != 0 {
		fmt.Println()
		fmt.Println(text.Bold(text.Red(arrow)), text.Bold("Package conflicts found:"))

		for name, pkgs := range conflicts {
			str := text.Red(text.Bold(smallArrow)) + " Installing " + text.Cyan(name) + " will remove:"
			for pkg := range pkgs {
				str += " " + text.Cyan(pkg) + ","
			}
			str = strings.TrimSuffix(str, ",")

			fmt.Println(str)
		}

	}

	// Add the inner conflicts to the conflicts
	// These are used to decide what to pass --ask to (if set) or don't pass --noconfirm to
	// As we have no idea what the order is yet we add every inner conflict to the slice
	for name, pkgs := range innerConflicts {
		conflicts[name] = make(types.StringSet)
		for pkg := range pkgs {
			conflicts[pkg] = make(types.StringSet)
		}
	}

	if len(conflicts) > 0 {
		if !ask {
			if noconfirm {
				return nil, fmt.Errorf("Package conflicts can not be resolved with noconfirm, aborting")
			}

			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, text.Bold(text.Red(arrow)), text.Bold("Conflicting packages will have to be confirmed manually"))
			fmt.Fprintln(os.Stderr)
		}
	}

	return conflicts, nil
}

type missing struct {
	Good    types.StringSet
	Missing map[string][][]string
}

func stringSliceEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func (dp *Pool) checkMissing(dep string, stack []string, missing *missing) {
	if missing.Good.Get(dep) {
		return
	}

	if trees, ok := missing.Missing[dep]; ok {
		for _, tree := range trees {
			if stringSliceEqual(tree, stack) {
				return
			}
		}
		missing.Missing[dep] = append(missing.Missing[dep], stack)
		return
	}

	aurPkg := dp.findSatisfierAur(dep)
	if aurPkg != nil {
		missing.Good.Set(dep)
		for _, deps := range [3][]string{aurPkg.Depends, aurPkg.MakeDepends, aurPkg.CheckDepends} {
			for _, aurDep := range deps {
				if _, err := dp.LocalDB.PkgCache().FindSatisfier(aurDep); err == nil {
					missing.Good.Set(aurDep)
					continue
				}

				dp.checkMissing(aurDep, append(stack, aurPkg.Name), missing)
			}
		}

		return
	}

	repoPkg := dp.findSatisfierRepo(dep)
	if repoPkg != nil {
		missing.Good.Set(dep)
		repoPkg.Depends().ForEach(func(repoDep alpm.Depend) error {
			if _, err := dp.LocalDB.PkgCache().FindSatisfier(repoDep.String()); err == nil {
				missing.Good.Set(repoDep.String())
				return nil
			}

			dp.checkMissing(repoDep.String(), append(stack, repoPkg.Name()), missing)
			return nil
		})

		return
	}

	missing.Missing[dep] = [][]string{stack}
}

func (dp *Pool) CheckMissing() error {
	missing := &missing{
		make(types.StringSet),
		make(map[string][][]string),
	}

	for _, target := range dp.Targets {
		dp.checkMissing(target.DepString(), make([]string, 0), missing)
	}

	if len(missing.Missing) == 0 {
		return nil
	}

	fmt.Println(text.Bold(text.Red(arrow+" Error: ")) + "Could not find all required packages:")
	for dep, trees := range missing.Missing {
		for _, tree := range trees {

			fmt.Print("    ", text.Cyan(dep))

			if len(tree) == 0 {
				fmt.Print(" (Target")
			} else {
				fmt.Print(" (Wanted by: ")
				for n := 0; n < len(tree)-1; n++ {
					fmt.Print(text.Cyan(tree[n]), " -> ")
				}
				fmt.Print(text.Cyan(tree[len(tree)-1]))
			}

			fmt.Println(")")
		}
	}

	return fmt.Errorf("")
}
