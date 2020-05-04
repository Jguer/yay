package text

import (
	"fmt"
	"os"
)

const arrow = "==>"
const smallArrow = " ->"
const opSymbol = ":: "

func OperationInfoln(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{boldCode, cyan(opSymbol), boldCode}, a...)...)
	fmt.Fprintln(os.Stdout, resetCode)
}

func OperationInfo(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{boldCode, cyan(opSymbol), boldCode}, a...)...)
	fmt.Fprint(os.Stdout, resetCode+" ")
}

func Info(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{bold(green(arrow + " "))}, a...)...)
}

func Infoln(a ...interface{}) {
	fmt.Fprintln(os.Stdout, append([]interface{}{bold(green(arrow))}, a...)...)
}

func Warn(a ...interface{}) {
	fmt.Fprint(os.Stdout, append([]interface{}{bold(yellow(smallArrow))}, a...)...)
}

func Warnln(a ...interface{}) {
	fmt.Fprintln(os.Stdout, append([]interface{}{bold(yellow(smallArrow))}, a...)...)
}

func Error(a ...interface{}) {
	fmt.Fprint(os.Stderr, append([]interface{}{bold(red(smallArrow))}, a...)...)
}

func Errorln(a ...interface{}) {
	fmt.Fprintln(os.Stderr, append([]interface{}{bold(red(smallArrow))}, a...)...)
}

func PrintInfoValue(str, value string) {
	if value == "" {
		value = "None"
	}

	fmt.Fprintln(os.Stdout, bold("%-16s%s")+" %s\n", str, ":", value)
}
