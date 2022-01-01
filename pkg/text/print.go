package text

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"regexp"
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

func formatInfoValue(keyLength int, maxCols int, startCols int, value string) (int, string) {
	str := ""
	re := regexp.MustCompile(`(\s|[^\s]*)`)
	parts := re.FindAllString(value, -1)

	if len(parts) == 0 {
		return startCols, str
	}

	str += parts[0]
	cols := startCols + len(parts[0])

	for _, part := range parts[1:] {
		if part == "\n" {
			cols = keyLength
			str += "\n" + strings.Repeat(" ", keyLength)
		} else {
			if maxCols > keyLength && cols+len(part) >= maxCols {
				cols = keyLength
				str += "\n" + strings.Repeat(" ", keyLength)
			}
			if strings.IndexFunc(part, unicode.IsSpace) == -1 {
				str += part
				cols += len(part)
			} else if cols != keyLength {
				str += " "
				cols += 1
			}
		}
	}

	return cols, str
}

func PrintInfoValue(key string, values ...string) {
	// 16 (text) + 1 (:) + 1 ( )
	const (
		keyLength  = 18
		delimCount = 2
	)

	str := fmt.Sprintf(Bold("%-16s: "), key)
	if len(values) == 0 || (len(values) == 1 && values[0] == "") {
		fmt.Fprintf(os.Stdout, "%s%s\n", str, gotext.Get("None"))
		return
	}

	maxCols := getColumnCount()
	cols, formattedValue := formatInfoValue(keyLength, maxCols, keyLength, values[0])
	str += formattedValue

	for _, value := range values[1:] {
		if maxCols > keyLength && cols+len(value)+delimCount >= maxCols {
			cols = keyLength
			str += "\n" + strings.Repeat(" ", keyLength)
		} else if cols != keyLength {
			str += strings.Repeat(" ", delimCount)
			cols += delimCount
		}

		cols, formattedValue = formatInfoValue(keyLength, maxCols, cols, value)
		str += formattedValue
	}

	fmt.Println(str)
}
