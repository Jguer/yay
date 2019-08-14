package text

import "fmt"

const (
	redCode     = "\x1b[31m"
	greenCode   = "\x1b[32m"
	yellowCode  = "\x1b[33m"
	blueCode    = "\x1b[34m"
	magentaCode = "\x1b[35m"
	cyanCode    = "\x1b[36m"
	boldCode    = "\x1b[1m"
	resetCode   = "\x1b[0m"
)

// UseColor determines if package will emit colors
var UseColor = true

// ColorHash Colors text using a hashing algorithm. The same text will always produce the
// same colour while different text will produce a different colour.
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

func stylize(startCode, in string) string {
	if UseColor {
		return startCode + in + resetCode
	}

	return in
}

func Red(in string) string {
	return stylize(redCode, in)
}

func Green(in string) string {
	return stylize(greenCode, in)
}

func Yellow(in string) string {
	return stylize(yellowCode, in)
}

func Blue(in string) string {
	return stylize(blueCode, in)
}

func Cyan(in string) string {
	return stylize(cyanCode, in)
}

func Magenta(in string) string {
	return stylize(magentaCode, in)
}

func Bold(in string) string {
	return stylize(boldCode, in)
}
