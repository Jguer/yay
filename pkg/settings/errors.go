package settings

import "fmt"

type ErrPrivilegeElevatorNotFound struct {
	confValue string
}

func (e *ErrPrivilegeElevatorNotFound) Error() string {
	return fmt.Sprintf("unable to find a privilege elevator, config value: %s", e.confValue)
}
