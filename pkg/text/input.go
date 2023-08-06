package text

import (
	"bufio"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/leonelquinteros/gotext"
)

func (l *Logger) GetInput(defaultValue string, noConfirm bool) (string, error) {
	l.Info()

	if defaultValue != "" || noConfirm {
		l.Println(defaultValue)
		return defaultValue, nil
	}

	reader := bufio.NewReader(l.r)

	buf, overflow, err := reader.ReadLine()
	if err != nil {
		return "", err
	}

	if overflow {
		return "", ErrInputOverflow{}
	}

	return string(buf), nil
}

// ContinueTask prompts if user wants to continue task.
// If NoConfirm is set the action will continue without user input.
func (l *Logger) ContinueTask(s string, preset, noConfirm bool) bool {
	if noConfirm {
		return preset
	}

	var (
		response string
		postFix  string
		n        string
		y        string
		yes      = gotext.Get("yes")
		no       = gotext.Get("no")
	)

	// Only use localized "y" and "n" if they are latin characters.
	if nRune, _ := utf8.DecodeRuneInString(no); unicode.Is(unicode.Latin, nRune) {
		n = string(nRune)
	} else {
		n = nDefault
	}

	if yRune, _ := utf8.DecodeRuneInString(yes); unicode.Is(unicode.Latin, yRune) {
		y = string(yRune)
	} else {
		y = yDefault
	}

	if preset { // If default behavior is true, use y as default.
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else { // If default behavior is anything else, use n as default.
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	l.OperationInfo(Bold(s), Bold(postFix))

	if _, err := fmt.Fscanln(l.r, &response); err != nil {
		return preset
	}

	return strings.EqualFold(response, yes) ||
		strings.EqualFold(response, y) ||
		(!strings.EqualFold(yDefault, n) && strings.EqualFold(response, yDefault))
}
