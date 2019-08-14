package query

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	rpc "github.com/mikkeloscar/aur"
)

// AUR is a collection of Results
type AUR []rpc.Pkg

func (q AUR) Len() int {
	return len(q)
}

func (q AUR) Less(i, j int, sortBy string, sortMode int) bool {
	var result bool

	switch sortBy {
	case "votes":
		result = q[i].NumVotes > q[j].NumVotes
	case "popularity":
		result = q[i].Popularity > q[j].Popularity
	case "name":
		result = types.LessRunes([]rune(q[i].Name), []rune(q[j].Name))
	case "base":
		result = types.LessRunes([]rune(q[i].PackageBase), []rune(q[j].PackageBase))
	case "submitted":
		result = q[i].FirstSubmitted < q[j].FirstSubmitted
	case "modified":
		result = q[i].LastModified < q[j].LastModified
	case "id":
		result = q[i].ID < q[j].ID
	case "baseid":
		result = q[i].PackageBaseID < q[j].PackageBaseID
	}

	if sortMode == runtime.BottomUp {
		return !result
	}

	return result
}

func (q AUR) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// PrintSearch handles printing search results in a given format
func (q AUR) PrintSearch(alpmHandle *alpm.Handle, searchMode int, sortMode int, start int) {
	localDB, _ := alpmHandle.LocalDB()

	for i, res := range q {
		var toprint string
		if searchMode == runtime.NumberMenu {
			switch sortMode {
			case runtime.TopDown:
				toprint += text.Magenta(strconv.Itoa(start+i) + " ")
			case runtime.BottomUp:
				toprint += text.Magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			default:
				fmt.Println("Invalid Sort Mode. Fix with yay -Y --bottomup --save")
			}
		} else if searchMode == runtime.Minimal {
			fmt.Println(res.Name)
			continue
		}

		toprint += text.Bold(text.ColorHash("aur")) + "/" + text.Bold(res.Name) +
			" " + text.Cyan(res.Version) +
			text.Bold(" (+"+strconv.Itoa(res.NumVotes)) +
			" " + text.Bold(strconv.FormatFloat(res.Popularity, 'f', 2, 64)+"%) ")

		if res.Maintainer == "" {
			toprint += text.Bold(text.Red("(Orphaned)")) + " "
		}

		if res.OutOfDate != 0 {
			toprint += text.Bold(text.Red("(Out-of-date "+text.FormatTime(res.OutOfDate)+")")) + " "
		}

		if pkg := localDB.Pkg(res.Name); pkg != nil {
			if pkg.Version() != res.Version {
				toprint += text.Bold(text.Green("(Installed: " + pkg.Version() + ")"))
			} else {
				toprint += text.Bold(text.Green("(Installed)"))
			}
		}
		toprint += "\n    " + res.Description
		fmt.Println(toprint)
	}
}

// AURNarrow searches AUR and narrows based on subarguments
func AURNarrow(pkgS []string, sortS bool, sortBy string, sortMode int) (AUR, error) {
	var r []rpc.Pkg
	var err error
	var usedIndex int

	if len(pkgS) == 0 {
		return nil, nil
	}

	for i, word := range pkgS {
		r, err = rpc.Search(word)
		if err == nil {
			usedIndex = i
			break
		}
	}

	if err != nil {
		return nil, err
	}

	if len(pkgS) == 1 {
		if sortS {
			sort.Slice(r, func(i, j int) bool {
				return AUR(r).Less(i, j, sortBy, sortMode)
			})
		}
		return r, err
	}

	var aq AUR
	var n int

	for _, res := range r {
		match := true
		for i, pkgN := range pkgS {
			if usedIndex == i {
				continue
			}

			if !(strings.Contains(res.Name, pkgN) || strings.Contains(strings.ToLower(res.Description), pkgN)) {
				match = false
				break
			}
		}

		if match {
			n++
			aq = append(aq, res)
		}
	}

	if sortS {
		sort.Slice(aq, func(i, j int) bool {
			return AUR(aq).Less(i, j, sortBy, sortMode)
		})
	}

	return aq, err
}

// AURInfo queries the aur for information about specified packages.
// All packages should be queried in a single rpc request except when the number
// of packages exceeds the number set in config.RequestSplitN.
// If the number does exceed config.RequestSplitN multiple rpc requests will be
// performed concurrently.
func AURInfo(config *runtime.Configuration, names []string, warnings *types.AURWarnings) ([]*rpc.Pkg, error) {
	info := make([]*rpc.Pkg, 0, len(names))
	seen := make(map[string]int)
	var mux sync.Mutex
	var wg sync.WaitGroup
	var errs types.MultiError

	makeRequest := func(n, max int) {
		defer wg.Done()
		tempInfo, requestErr := rpc.Info(names[n:max])
		errs.Add(requestErr)
		if requestErr != nil {
			return
		}
		mux.Lock()
		for _, _i := range tempInfo {
			i := _i
			info = append(info, &i)
		}
		mux.Unlock()
	}

	for n := 0; n < len(names); n += config.RequestSplitN {
		max := types.Min(len(names), n+config.RequestSplitN)
		wg.Add(1)
		go makeRequest(n, max)
	}

	wg.Wait()

	if err := errs.Return(); err != nil {
		return info, err
	}

	for k, pkg := range info {
		seen[pkg.Name] = k
	}

	for _, name := range names {
		i, ok := seen[name]
		if !ok {
			warnings.Missing = append(warnings.Missing, name)
			continue
		}

		pkg := info[i]

		if pkg.Maintainer == "" {
			warnings.Orphans = append(warnings.Orphans, name)
		}
		if pkg.OutOfDate != 0 {
			warnings.OutOfDate = append(warnings.OutOfDate, name)
		}
	}

	return info, nil
}

// AURInfoPrint wraps AURInfo for user display
func AURInfoPrint(config *runtime.Configuration, names []string) ([]*rpc.Pkg, error) {
	fmt.Println(text.Bold(text.Cyan("::") + text.Bold(" Querying AUR...")))

	warnings := &types.AURWarnings{}
	info, err := AURInfo(config, names, warnings)
	if err != nil {
		return info, err
	}

	warnings.Print()

	return info, nil
}
