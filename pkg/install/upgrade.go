package install

import (
	"fmt"
	"sort"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/lookup/upgrade"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

// upgradePkgs handles updating the cache and installing updates.
func upgradePkgs(config *runtime.Configuration, alpmHandle *alpm.Handle, aurUp, repoUp upgrade.UpSlice) (types.StringSet, types.StringSet, error) {
	ignore := make(types.StringSet)
	aurNames := make(types.StringSet)

	allUpLen := len(repoUp) + len(aurUp)
	if allUpLen == 0 {
		return ignore, aurNames, nil
	}

	if !config.UpgradeMenu {
		for _, pkg := range aurUp {
			aurNames.Set(pkg.Name)
		}

		return ignore, aurNames, nil
	}

	sort.Slice(repoUp, func(i, j int) bool {
		return repoUp.Less(i, j, alpmHandle)
	})

	sort.Slice(aurUp, func(i, j int) bool {
		return aurUp.Less(i, j, alpmHandle)
	})

	allUp := append(repoUp, aurUp...)
	fmt.Printf("%s"+text.Bold(" %d ")+"%s\n", text.Bold(text.Cyan("::")), allUpLen, text.Bold("Packages to upgrade."))
	allUp.Print()

	fmt.Println(text.Bold(text.Green(arrow + " Packages to not upgrade: (eg: 1 2 3, 1-3, ^4 or repo name)")))
	fmt.Print(text.Bold(text.Green(arrow + " ")))

	numbers, err := text.GetInput(config.AnswerUpgrade, config.NoConfirm)
	if err != nil {
		return nil, nil, err
	}

	//upgrade menu asks you which packages to NOT upgrade so in this case
	//include and exclude are kind of swapped
	//include, exclude, other := parseNumberMenu(string(numberBuf))
	include, exclude, otherInclude, otherExclude := types.ParseNumberMenu(numbers)

	isInclude := len(exclude) == 0 && len(otherExclude) == 0

	for i, pkg := range repoUp {
		if isInclude && otherInclude.Get(pkg.Repository) {
			ignore.Set(pkg.Name)
		}

		if isInclude && !include.Get(len(repoUp)-i+len(aurUp)) {
			continue
		}

		if !isInclude && (exclude.Get(len(repoUp)-i+len(aurUp)) || otherExclude.Get(pkg.Repository)) {
			continue
		}

		ignore.Set(pkg.Name)
	}

	for i, pkg := range aurUp {
		if isInclude && otherInclude.Get(pkg.Repository) {
			continue
		}

		if isInclude && !include.Get(len(aurUp)-i) {
			aurNames.Set(pkg.Name)
		}

		if !isInclude && (exclude.Get(len(aurUp)-i) || otherExclude.Get(pkg.Repository)) {
			aurNames.Set(pkg.Name)
		}
	}

	return ignore, aurNames, err
}
