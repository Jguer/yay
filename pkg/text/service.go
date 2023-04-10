package text

import (
	"fmt"
	"io"
)

type Logger struct {
	name   string
	Debug  bool
	stdout io.Writer
	stderr io.Writer
	r      io.Reader
}

func NewLogger(stdout, stderr io.Writer, r io.Reader, debug bool, name string) *Logger {
	return &Logger{
		Debug:  debug,
		name:   name,
		r:      r,
		stderr: stderr,
		stdout: stdout,
	}
}

func (l *Logger) Child(name string) *Logger {
	return NewLogger(l.stdout, l.stderr, l.r, l.Debug, name)
}

func (l *Logger) Debugln(a ...any) {
	if !l.Debug {
		return
	}

	l.Println(append([]interface{}{
		Bold(yellow(fmt.Sprintf("[DEBUG:%s]", l.name))),
	}, a...)...)
}

func (l *Logger) OperationInfoln(a ...any) {
	l.Println(l.SprintOperationInfo(a...))
}

func (l *Logger) OperationInfo(a ...any) {
	l.Print(l.SprintOperationInfo(a...))
}

func (l *Logger) SprintOperationInfo(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(Cyan(opSymbol + " ")), boldCode}, a...)...) + ResetCode
}

func (l *Logger) Info(a ...any) {
	l.Print(append([]interface{}{Bold(Green(arrow + " "))}, a...)...)
}

func (l *Logger) Infoln(a ...any) {
	l.Println(append([]interface{}{Bold(Green(arrow))}, a...)...)
}

func (l *Logger) Warn(a ...any) {
	l.Print(l.SprintWarn(a...))
}

func (l *Logger) Warnln(a ...any) {
	l.Println(l.SprintWarn(a...))
}

func (l *Logger) SprintWarn(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(yellow(smallArrow + " "))}, a...)...)
}

func (l *Logger) Error(a ...any) {
	fmt.Fprint(l.stderr, l.SprintError(a...))
}

func (l *Logger) Errorln(a ...any) {
	fmt.Fprintln(l.stderr, l.SprintError(a...))
}

func (l *Logger) SprintError(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(Red(smallArrow + " "))}, a...)...)
}

func (l *Logger) Printf(format string, a ...any) {
	fmt.Fprintf(l.stdout, format, a...)
}

func (l *Logger) Println(a ...any) {
	fmt.Fprintln(l.stdout, a...)
}

func (l *Logger) Print(a ...any) {
	fmt.Fprint(l.stdout, a...)
}
