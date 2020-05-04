package text

import "fmt"

const (
	redCode    = "\x1b[31m"
	greenCode  = "\x1b[32m"
	yellowCode = "\x1b[33m"
	cyanCode   = "\x1b[36m"
	boldCode   = "\x1b[1m"

	resetCode = "\x1b[0m"
)

// UseColor determines if package will emit colors
var UseColor = true

func stylize(startCode, in string) string {
	if UseColor {
		return startCode + in + resetCode
	}

	return in
}

func red(in string) string {
	return stylize(redCode, in)
}

func green(in string) string {
	return stylize(greenCode, in)
}

func yellow(in string) string {
	return stylize(yellowCode, in)
}

func cyan(in string) string {
	return stylize(cyanCode, in)
}

func bold(in string) string {
	return stylize(boldCode, in)
}

// ColorHash Colors text using a hashing algorithm. The same text will always produce the
// same color while different text will produce a different color.
func ColorHash(name string) (output string) {
	if !UseColor {
		return name
	}
	var hash uint = 5381
	for i := 0; i < len(name); i++ {
		hash = uint(name[i]) + ((hash << 5) + (hash))
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", hash%6+31, name)
}
