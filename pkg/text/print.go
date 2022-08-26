package text

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"github.com/leonelquinteros/gotext"
	"golang.org/x/sys/unix"
)

const (
	arrow      = "==>"
	smallArrow = " ->"
	opSymbol   = "::"
)

var cachedColumnCount = -1

func OperationInfoln(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{Bold(Cyan(opSymbol + " ")), boldCode}, a...)...)
	fmt.Fprintln(os.Stdout, ResetCode)
}

func OperationInfo(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{Bold(Cyan(opSymbol + " ")), boldCode}, a...)...)
	fmt.Fprint(os.Stdout, ResetCode)
}

func SprintOperationInfo(a ...interface{}) string {
	return fmt.Sprint(append([]interface{}{Bold(Cyan(opSymbol + " ")), boldCode}, a...)...) + ResetCode
}

func Info(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{Bold(Green(arrow + " "))}, a...)...)
}

func Infoln(a ...interface{}) {
	fmt.Fprintln(os.Stdout, append([]interface{}{Bold(Green(arrow))}, a...)...)
}

func SprintWarn(a ...interface{}) string {
	return fmt.Sprint(append([]interface{}{Bold(yellow(smallArrow + " "))}, a...)...)
}

func Warn(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{Bold(yellow(smallArrow + " "))}, a...)...)
}

func Warnln(a ...interface{}) {
	fmt.Fprintln(os.Stdout, append([]interface{}{Bold(yellow(smallArrow))}, a...)...)
}

func SprintError(a ...interface{}) string {
	return fmt.Sprint(append([]interface{}{Bold(Red(smallArrow + " "))}, a...)...)
}

func Error(a ...interface{}) {
	fmt.Fprint(os.Stderr, append([]interface{}{Bold(Red(smallArrow + " "))}, a...)...)
}

func Errorln(a ...interface{}) {
	fmt.Fprintln(os.Stderr, append([]interface{}{Bold(Red(smallArrow))}, a...)...)
}

func getColumnCount() int {
	if cachedColumnCount > 0 {
		return cachedColumnCount
	}

	if count, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil {
		cachedColumnCount = count
		return cachedColumnCount
	}

	if ws, err := unix.IoctlGetWinsize(syscall.Stdout, unix.TIOCGWINSZ); err == nil {
		cachedColumnCount = int(ws.Col)
		return cachedColumnCount
	}

	return 80
}

func PrintInfoValue(key string, values ...string) {
	const (
		keyLength  = 32
		delimCount = 2
	)

	specialWordsCount := 0

	for _, runeValue := range key {
		// CJK handling: the character 'ー' is Katakana
		// but if use unicode.Katakana, it will return false
		if unicode.IsOneOf([]*unicode.RangeTable{
			unicode.Han,
			unicode.Hiragana,
			unicode.Katakana,
			unicode.Hangul,
		}, runeValue) || runeValue == 'ー' {
			specialWordsCount++
		}
	}

	keyTextCount := specialWordsCount - keyLength + delimCount
	str := fmt.Sprintf(Bold("%-*s: "), keyTextCount, key)

	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		fmt.Fprintf(os.Stdout, "%s%s\n", str, gotext.Get("None"))
		return
	}

	maxCols := getColumnCount()
	cols := keyLength + len(values[0])
	str += values[0]

	for _, value := range values[1:] {
		if maxCols > keyLength && cols+len(value)+delimCount >= maxCols {
			cols = keyLength
			str += "\n" + strings.Repeat(" ", keyLength)
		} else if cols != keyLength {
			str += strings.Repeat(" ", delimCount)
			cols += delimCount
		}

		str += value
		cols += len(value)
	}

	fmt.Println(str)
}
