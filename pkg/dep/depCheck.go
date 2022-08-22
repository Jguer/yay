package dep

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
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

		if satisfiesRepo(conflict, pkg, dp.AlpmExecutor) {
			conflicts.Add(name, pkg.Name())
		}
	}
}

func (dp *Pool) checkForwardConflict(name, conflict string, conflicts stringset.MapStringSet) {
	for _, pkg := range dp.AlpmExecutor.LocalPackages() {
		if pkg.Name() == name || dp.hasPackage(pkg.Name()) {
			continue
		}

		if satisfiesRepo(conflict, pkg, dp.AlpmExecutor) {
			n := pkg.Name()
			if n != conflict {
				n += " (" + conflict + ")"
			}

			conflicts.Add(name, n)
		}
	}
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

		if satisfiesRepo(conflict, pkg, dp.AlpmExecutor) {
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
		for _, conflict := range dp.AlpmExecutor.PackageConflicts(pkg) {
			dp.checkInnerConflict(pkg.Name(), conflict.String(), conflicts)
		}
	}
}

func (dp *Pool) checkForwardConflicts(conflicts stringset.MapStringSet) {
	for _, pkg := range dp.Aur {
		for _, conflict := range pkg.Conflicts {
			dp.checkForwardConflict(pkg.Name, conflict, conflicts)
		}
	}

	for _, pkg := range dp.Repo {
		for _, conflict := range dp.AlpmExecutor.PackageConflicts(pkg) {
			dp.checkForwardConflict(pkg.Name(), conflict.String(), conflicts)
		}
	}
}

func (dp *Pool) checkReverseConflicts(conflicts stringset.MapStringSet) {
	for _, pkg := range dp.AlpmExecutor.LocalPackages() {
		if dp.hasPackage(pkg.Name()) {
			continue
		}

		for _, conflict := range dp.AlpmExecutor.PackageConflicts(pkg) {
			dp.checkReverseConflict(pkg.Name(), conflict.String(), conflicts)
		}
	}
}

func (dp *Pool) CheckConflicts(useAsk, noConfirm, noDeps bool) (stringset.MapStringSet, error) {
	conflicts := make(stringset.MapStringSet)
	if noDeps {
		return conflicts, nil
	}

	var wg sync.WaitGroup

	innerConflicts := make(stringset.MapStringSet)

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
		text.Errorln(gotext.Get("Inner conflicts found:"))

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
		text.Errorln(gotext.Get("Package conflicts found:"))

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
				return nil, errors.New(gotext.Get("package conflicts can not be resolved with noconfirm, aborting"))
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

func (dp *Pool) _checkMissing(dep string, stack []string, missing *missing, noDeps, noCheckDeps bool) {
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

	if aurPkg := dp.findSatisfierAur(dep); aurPkg != nil {
		missing.Good.Set(dep)

		combinedDepList := ComputeCombinedDepList(aurPkg, noDeps, noCheckDeps)
		for _, aurDep := range combinedDepList {
			if dp.AlpmExecutor.LocalSatisfierExists(aurDep) {
				missing.Good.Set(aurDep)
				continue
			}

			dp._checkMissing(aurDep, append(stack, aurPkg.Name), missing, noDeps, noCheckDeps)
		}

		return
	}

	if repoPkg := dp.findSatisfierRepo(dep); repoPkg != nil {
		missing.Good.Set(dep)

		if noDeps {
			return
		}

		for _, dep := range dp.AlpmExecutor.PackageDepends(repoPkg) {
			if dp.AlpmExecutor.LocalSatisfierExists(dep.String()) {
				missing.Good.Set(dep.String())
				continue
			}

			dp._checkMissing(dep.String(), append(stack, repoPkg.Name()), missing, noDeps, noCheckDeps)
		}

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

func (dp *Pool) CheckMissing(noDeps, noCheckDeps bool) error {
	missing := &missing{
		make(stringset.StringSet),
		make(map[string][][]string),
	}

	for _, target := range dp.Targets {
		dp._checkMissing(target.DepString(), make([]string, 0), missing, noDeps, noCheckDeps)
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
