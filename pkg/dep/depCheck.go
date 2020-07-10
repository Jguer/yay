package dep

import (
	"fmt"
	"os"
	"strings"
	"sync"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/stringset"
	"github.com/Jguer/yay/v10/pkg/text"
)

func (dp *Pool) checkInnerConflict(name, conflict string, conflicts stringset.MapStringSet) {
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

func (dp *Pool) checkForwardConflict(name, conflict string, conflicts stringset.MapStringSet) {
	_ = dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
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

func (dp *Pool) checkReverseConflict(name, conflict string, conflicts stringset.MapStringSet) {
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

func (dp *Pool) checkInnerConflicts(conflicts stringset.MapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkInnerConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		_ = pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkInnerConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *Pool) checkForwardConflicts(conflicts stringset.MapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkForwardConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		_ = pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkForwardConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})
	}
}

func (dp *Pool) checkReverseConflicts(conflicts stringset.MapStringSet) {
	_ = dp.LocalDB.PkgCache().ForEach(func(pkg alpm.Package) error {
		if dp.hasPackage(pkg.Name()) {
			return nil
		}

		_ = pkg.Conflicts().ForEach(func(conflict alpm.Depend) error {
			dp.checkReverseConflict(pkg.Name(), conflict.String(), conflicts)
			return nil
		})

		return nil
	})
}

func (dp *Pool) CheckConflicts(useAsk, noConfirm bool) (stringset.MapStringSet, error) {
	var wg sync.WaitGroup
	innerConflicts := make(stringset.MapStringSet)
	conflicts := make(stringset.MapStringSet)
	wg.Add(2)

	text.OperationInfoln(gotext.Get("Checking for conflicts..."))
	go func() {
		dp.checkForwardConflicts(conflicts)
		dp.checkReverseConflicts(conflicts)
		wg.Done()
	}()

	text.OperationInfoln(gotext.Get("Checking for inner conflicts..."))
	go func() {
		dp.checkInnerConflicts(innerConflicts)
		wg.Done()
	}()

	wg.Wait()

	if len(innerConflicts) != 0 {
		text.Errorln(gotext.Get("\nInner conflicts found:"))

		for name, pkgs := range innerConflicts {
			str := text.SprintError(name + ":")
			for pkg := range pkgs {
				str += " " + text.Cyan(pkg) + ","
			}
			str = strings.TrimSuffix(str, ",")

			fmt.Println(str)
		}
	}

	if len(conflicts) != 0 {
		text.Errorln(gotext.Get("\nPackage conflicts found:"))

		for name, pkgs := range conflicts {
			str := text.SprintError(gotext.Get("Installing %s will remove:", text.Cyan(name)))
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
		conflicts[name] = make(stringset.StringSet)
		for pkg := range pkgs {
			conflicts[pkg] = make(stringset.StringSet)
		}
	}

	if len(conflicts) > 0 {
		if !useAsk {
			if noConfirm {
				return nil, fmt.Errorf(gotext.Get("package conflicts can not be resolved with noconfirm, aborting"))
			}

			text.Errorln(gotext.Get("Conflicting packages will have to be confirmed manually"))
		}
	}

	return conflicts, nil
}

type missing struct {
	Good    stringset.StringSet
	Missing map[string][][]string
}

func (dp *Pool) _checkMissing(dep string, stack []string, missing *missing) {
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

				dp._checkMissing(aurDep, append(stack, aurPkg.Name), missing)
			}
		}

		return
	}

	repoPkg := dp.findSatisfierRepo(dep)
	if repoPkg != nil {
		missing.Good.Set(dep)
		_ = repoPkg.Depends().ForEach(func(repoDep alpm.Depend) error {
			if _, err := dp.LocalDB.PkgCache().FindSatisfier(repoDep.String()); err == nil {
				missing.Good.Set(repoDep.String())
				return nil
			}

			dp._checkMissing(repoDep.String(), append(stack, repoPkg.Name()), missing)
			return nil
		})

		return
	}

	missing.Missing[dep] = [][]string{stack}
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

func (dp *Pool) CheckMissing() error {
	missing := &missing{
		make(stringset.StringSet),
		make(map[string][][]string),
	}

	for _, target := range dp.Targets {
		dp._checkMissing(target.DepString(), make([]string, 0), missing)
	}

	if len(missing.Missing) == 0 {
		return nil
	}

	text.Errorln(gotext.Get("Could not find all required packages:"))
	for dep, trees := range missing.Missing {
		for _, tree := range trees {
			fmt.Fprintf(os.Stderr, "\t%s", text.Cyan(dep))

			if len(tree) == 0 {
				fmt.Fprint(os.Stderr, gotext.Get(" (Target"))
			} else {
				fmt.Fprint(os.Stderr, gotext.Get(" (Wanted by: "))
				for n := 0; n < len(tree)-1; n++ {
					fmt.Fprint(os.Stderr, text.Cyan(tree[n]), " -> ")
				}
				fmt.Fprint(os.Stderr, text.Cyan(tree[len(tree)-1]))
			}

			fmt.Fprintln(os.Stderr, ")")
		}
	}

	return fmt.Errorf("")
}
