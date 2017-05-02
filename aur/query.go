package aur

import (
	"fmt"

	"github.com/jguer/yay/util"
	rpc "github.com/mikkeloscar/aur"
)

const aurURL = "https://aur.archlinux.org/rpc/?"

// Query is a collection of Results
type Query []rpc.Pkg

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
