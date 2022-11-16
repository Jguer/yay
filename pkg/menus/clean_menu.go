// Clean Build Menu functions
package menus

import (
	"fmt"
	"io"
	"os"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/text"
)

func anyExistInCache(pkgbuildDirs map[string]string) bool {
	for _, dir := range pkgbuildDirs {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func Clean(w io.Writer, cleanMenuOption bool, pkgbuildDirs map[string]string,
	installed mapset.Set[string], noConfirm bool, answerClean string,
) error {
	if !(cleanMenuOption && anyExistInCache(pkgbuildDirs)) {
		return nil
	}

	skipFunc := func(pkg string) bool {
		dir := pkgbuildDirs[pkg]
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return true
		}

		return false
	}

	bases := make([]string, 0, len(pkgbuildDirs))
	for pkg := range pkgbuildDirs {
		bases = append(bases, pkg)
	}

	toClean, errClean := selectionMenu(w, pkgbuildDirs, bases, installed, gotext.Get("Packages to cleanBuild?"),
		noConfirm, answerClean, skipFunc)
	if errClean != nil {
		return errClean
	}

	for i, base := range toClean {
		dir := pkgbuildDirs[base]
		text.OperationInfoln(gotext.Get("Deleting (%d/%d): %s", i+1, len(toClean), text.Cyan(dir)))

		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	return nil
}
