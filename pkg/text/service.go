package text

import (
	"fmt"
	"io"
)

type Logger struct {
	name  string
	debug bool
	w     io.Writer
	r     io.Reader
}

func NewLogger(w io.Writer, r io.Reader, debug bool, name string) *Logger {
	return &Logger{
		w:     w,
		name:  name,
		debug: debug,
		r:     r,
	}
}

func (l *Logger) Child(name string) *Logger {
	return NewLogger(l.w, l.r, l.debug, name)
}

func (l *Logger) Debugln(a ...any) {
	if !DebugMode {
		return
	}

	fmt.Fprintln(l.w, append([]interface{}{
		Bold(yellow(fmt.Sprintf("[DEBUG:%s]", l.name))),
	}, a...)...)
}

func (l *Logger) OperationInfoln(a ...any) {
	fmt.Fprintln(l.w, l.SprintOperationInfo(a...))
}

func (l *Logger) OperationInfo(a ...any) {
	fmt.Fprint(l.w, l.SprintOperationInfo(a...))
}

func (l *Logger) SprintOperationInfo(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(Cyan(opSymbol + " ")), boldCode}, a...)...) + ResetCode
}

func (l *Logger) Info(a ...any) {
	fmt.Fprint(l.w, append([]interface{}{Bold(Green(arrow + " "))}, a...)...)
}

func (l *Logger) Infoln(a ...any) {
	fmt.Fprintln(l.w, append([]interface{}{Bold(Green(arrow))}, a...)...)
}

func (l *Logger) Warn(a ...any) {
	fmt.Fprint(l.w, l.SprintWarn(a...))
}

func (l *Logger) Warnln(a ...any) {
	fmt.Fprintln(l.w, l.SprintWarn(a...))
}

func (l *Logger) SprintWarn(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(yellow(smallArrow + " "))}, a...)...)
}

func (l *Logger) Error(a ...any) {
	fmt.Fprint(l.w, l.SprintError(a...))
}

func (l *Logger) Errorln(a ...any) {
	fmt.Fprintln(l.w, l.SprintError(a...))
}

func (l *Logger) SprintError(a ...any) string {
	return fmt.Sprint(append([]interface{}{Bold(Red(smallArrow + " "))}, a...)...)
}

func (l *Logger) Printf(format string, a ...any) {
	fmt.Fprintf(l.w, format, a...)
}
