package aur

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jguer/yay/pacman"
	"github.com/jguer/yay/util"
)

// Query is a collection of Results
type Query []Result

func (q Query) Len() int {
	return len(q)
}

func (q Query) Less(i, j int) bool {
	if util.SortMode == util.BottomUp {
		return q[i].NumVotes < q[j].NumVotes
	}
	return q[i].NumVotes > q[j].NumVotes
}

func (q Query) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// PrintSearch handles printing search results in a given format
func (q Query) PrintSearch(start int) {
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

		if res.Installed == true {
			toprint += fmt.Sprintf("\x1b[32;40mInstalled\x1b[0m")
		}
		toprint += "\n" + res.Description
		fmt.Println(toprint)
	}

	return
}

// Info returns an AUR search with package details
func Info(pkg string) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}

	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=info&arg[]="+pkg, &r)

	return r.Results, r.ResultCount, err
}

// MultiInfo takes a slice of strings and returns a slice with the info of each package
func MultiInfo(pkgS []string) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}

	var pkg string
	for _, pkgn := range pkgS {
		pkg += "&arg[]=" + pkgn
	}

	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=info"+pkg, &r)

	return r.Results, r.ResultCount, err
}

// Search returns an AUR search
func Search(pkgS []string, sortS bool) (Query, int, error) {
	type returned struct {
		Results     Query `json:"results"`
		ResultCount int   `json:"resultcount"`
	}
	r := returned{}
	err := getJSON("https://aur.archlinux.org/rpc/?v=5&type=search&arg="+pkgS[0], &r)

	var aq Query
	n := 0
	setter := pacman.PFactory(pFSetTrue)
	var fri int

	for _, res := range r.Results {
		match := true
		for _, pkgN := range pkgS[1:] {
			if !(strings.Contains(res.Name, pkgN) || strings.Contains(strings.ToLower(res.Description), pkgN)) {
				match = false
				break
			}
		}

		if match {
			n++
			aq = append(aq, res)
			fri = len(aq) - 1
			setter(aq[fri].Name, &aq[fri], false)
		}
	}

	if aq != nil {
		setter(aq[fri].Name, &aq[fri], true)
	}

	if sortS {
		sort.Sort(aq)
	}

	return aq, n, err
}

// This is very dirty but it works so good.
func pFSetTrue(res interface{}) {
	f, ok := res.(*Result)
	if !ok {
		fmt.Println("Unable to convert back to Result")
		return
	}
	f.Installed = true

	return
}

// MissingPackage warns if the Query was unable to find a package
func (q Query) MissingPackage(pkgS []string) {
	for _, depName := range pkgS {
		found := false
		for _, dep := range q {
			if dep.Name == depName {
				found = true
				break
			}
		}

		if !found {
			fmt.Println("\x1b[31mUnable to find", depName, "in AUR\x1b[0m")
		}
	}
	return
}
