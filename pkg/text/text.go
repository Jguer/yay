package text

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/leonelquinteros/gotext"
)

// SplitDBFromName split apart db/package to db and package
func SplitDBFromName(pkg string) (db, name string) {
	split := strings.SplitN(pkg, "/", 2)

	if len(split) == 2 {
		return split[0], split[1]
	}
	return "", split[0]
}

// LessRunes compares two rune values, and returns true if the first argument is lexicographicaly smaller.
func LessRunes(iRunes, jRunes []rune) bool {
	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}

	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]

		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)

		if lir != ljr {
			return lir < ljr
		}

		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}

	return len(iRunes) < len(jRunes)
}

// ContinueTask prompts if user wants to continue task.
// If NoConfirm is set the action will continue without user input.
func ContinueTask(s string, cont, noConfirm bool) bool {
	if noConfirm {
		return cont
	}

	var response string
	var postFix string
	yes := gotext.Get("yes")
	no := gotext.Get("no")
	y := string([]rune(yes)[0])
	n := string([]rune(no)[0])

	if cont {
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else {
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	Info(Bold(s), Bold(postFix))

	if _, err := fmt.Scanln(&response); err != nil {
		return cont
	}

	response = strings.ToLower(response)
	return response == yes || response == y
}
