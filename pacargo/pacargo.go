package pacargo

import (
	"fmt"
	"os"
)

// ReturnArgs prints os args
func ReturnArgs() {
	for o := range os.Args {
		fmt.Println(o)
	}
}
