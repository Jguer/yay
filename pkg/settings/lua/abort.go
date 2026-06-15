package lua

import (
	"errors"

	glua "github.com/yuin/gopher-lua"
)

type abortError string

func (err abortError) Error() string {
	return string(err)
}

func (err abortError) String() string {
	return string(err)
}

func (abortError) Type() glua.LValueType {
	return glua.LTUserData
}

func abort(state *glua.LState) int {
	message := state.CheckString(1)
	state.Error(abortError(message), 0)

	return 0
}

func luaAbortError(err error) (abortError, bool) {
	var apiErr *glua.ApiError
	if !errors.As(err, &apiErr) {
		return "", false
	}

	abortErr, ok := apiErr.Object.(abortError)

	return abortErr, ok
}
