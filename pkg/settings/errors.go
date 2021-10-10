package settings

import (
	"fmt"

	"github.com/leonelquinteros/gotext"
)

type ErrPrivilegeElevatorNotFound struct {
	confValue string
}

func (e *ErrPrivilegeElevatorNotFound) Error() string {
	return fmt.Sprintf("unable to find a privilege elevator, config value: %s", e.confValue)
}

type ErrRuntimeDir struct {
	inner error
	dir   string
}

func (e *ErrRuntimeDir) Error() string {
	return gotext.Get("failed to create directory '%s': %s", e.dir, e.inner)
}

type ErrUserAbort struct{}

func (e ErrUserAbort) Error() string {
	return gotext.Get("aborting due to user")
}
