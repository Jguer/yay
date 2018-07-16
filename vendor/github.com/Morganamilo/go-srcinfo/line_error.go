package srcinfo

import (
	"fmt"
)

// LineError is an error type that stores the line number at which an error
// occurred as well the full Line that cased the error and an error string.
type LineError struct {
	LineNumber int    // The line number at which the error occurred
	Line       string // The line that caused the error
	ErrorStr   string // An error string
}

// Error Returns an error string in the format:
// "Line <LineNumber>: <ErrorStr>: <Line>".
func (le LineError) Error() string {
	return fmt.Sprintf("Line %d: %s: %s", le.LineNumber, le.ErrorStr, le.Line)
}

// Error Returns a new LineError
func Error(LineNumber int, Line string, ErrorStr string) *LineError {
	return &LineError{
		LineNumber,
		Line,
		ErrorStr,
	}
}

// Errorf Returns a new LineError using the same formatting rules as
// fmt.Printf.
func Errorf(LineNumber int, Line string, ErrorStr string, args ...interface{}) *LineError {
	return &LineError{
		LineNumber,
		Line,
		fmt.Sprintf(ErrorStr, args...),
	}
}
