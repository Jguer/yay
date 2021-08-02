package download

import (
	"fmt"

	"github.com/leonelquinteros/gotext"
)

// ErrAURPackageNotFound means that package was not found in AUR.
type ErrAURPackageNotFound struct {
	pkgName string
}

func (e ErrAURPackageNotFound) Error() string {
	return fmt.Sprintln(gotext.Get("package not found in AUR"), ":", e.pkgName)
}

type ErrGetPKGBUILDRepo struct {
	inner   error
	pkgName string
	errOut  string
}

func (e ErrGetPKGBUILDRepo) Error() string {
	return fmt.Sprintln(gotext.Get("error fetching %s: %s", e.pkgName, e.errOut),
		"\n\t context:", e.inner.Error())
}

func (e *ErrGetPKGBUILDRepo) Unwrap() error {
	return e.inner
}
