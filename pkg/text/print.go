package text

import (
	"fmt"
	"os"

	"github.com/leonelquinteros/gotext"
)

const (
	arrow      = "==>"
	smallArrow = " ->"
	opSymbol   = "::"
)

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
	fmt.Fprint(os.Stdout, append([]interface{}{Bold(green(arrow + " "))}, a...)...)
}

func Infoln(a ...interface{}) {
	fmt.Fprintln(os.Stdout, append([]interface{}{Bold(green(arrow))}, a...)...)
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

func PrintInfoValue(str, value string) {
	if value == "" {
		value = gotext.Get("None")
	}

	fmt.Fprintf(os.Stdout, Bold("%-16s%s")+" %s\n", str, ":", value)
}
