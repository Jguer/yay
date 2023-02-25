package text

import (
	"fmt"
	"io"
)

type Logger struct {
	name  string
	Debug bool
	w     io.Writer
	r     io.Reader
}

func NewLogger(w io.Writer, r io.Reader, debug bool, name string) *Logger {
	return &Logger{
		w:     w,
		name:  name,
		Debug: debug,
		r:     r,
	}
}

func (l *Logger) Child(name string) *Logger {
	return NewLogger(l.w, l.r, l.Debug, name)
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
	l.Print(l.SprintError(a...))
}

func (l *Logger) Errorln(a ...any) {
	l.Println(l.SprintError(a...))
}

func (l *Logger) SprintError(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(Red(smallArrow + " "))}, a...)...)
}

func (l *Logger) Printf(format string, a ...any) {
	fmt.Fprintf(l.w, format, a...)
}

func (l *Logger) Println(a ...any) {
	fmt.Fprintln(l.w, a...)
}

func (l *Logger) Print(a ...any) {
	fmt.Fprint(l.w, a...)
}
