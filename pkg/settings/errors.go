package settings

import (
	"fmt"
	"strings"
)

type UnknownOption struct {
	key string
}

func (io UnknownOption) Error() string {
	return fmt.Sprintf("unknown option '%s'", io.key)
}

type InvalidOption struct {
	key      string
	value    string
	expected []string
}

func IsInvalidOption(err error) bool {
	_, ok := err.(InvalidOption)
	return ok
}

func (io InvalidOption) Error() string {
	if io.value == "" {
		return fmt.Sprintf("option '%s' requires a value", io.key)
	}
	if len(io.expected) == 0 {
		return fmt.Sprintf("invalid value for '%s' : %s - expected no value",
			io.key, io.value)
	}
	return fmt.Sprintf("invalid value for '%s' : %s - expected %s",
		io.key, io.value, strings.Join(io.expected, "|"))
}
