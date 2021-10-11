// Clean Build Menu functions
package menus

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v11/pkg/dep"
	"github.com/Jguer/yay/v11/pkg/stringset"
	"github.com/Jguer/yay/v11/pkg/text"
)

func anyExistInCache(buildDir string, bases []dep.Base) bool {
	for _, base := range bases {
		pkg := base.Pkgbase()
		dir := filepath.Join(buildDir, pkg)

		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			return true
		}
	}

	return false
}

func Clean(cleanMenuOption bool, buildDir string, bases []dep.Base,
	installed stringset.StringSet, noConfirm bool, answerClean string) error {
	if !(cleanMenuOption && anyExistInCache(buildDir, bases)) {
		return nil
	}

	skipFunc := func(pkg string) bool {
		dir := filepath.Join(buildDir, pkg)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return true
		}

		return false
	}

	toClean, errClean := selectionMenu(buildDir, bases, installed, gotext.Get("Packages to cleanBuild?"),
		noConfirm, answerClean, skipFunc)
	if errClean != nil {
		return errClean
	}

	for i, base := range toClean {
		dir := filepath.Join(buildDir, base.Pkgbase())
		text.OperationInfoln(gotext.Get("Deleting (%d/%d): %s", i+1, len(toClean), text.Cyan(dir)))

		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}

	return nil
}
