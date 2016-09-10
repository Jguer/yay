package pacargo

import (
	"fmt"
	"os"
)

type operation struct {
	key         byte
	description string
	modifiers   []modifier
    execute func()
}

type modifier struct {
	description string
}

// ReturnArgs prints os args
func ReturnArgs() {
	for _, o := range os.Args {
		fmt.Println(o)
	}
}
