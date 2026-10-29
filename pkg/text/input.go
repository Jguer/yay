package text

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
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

	// The process-wide SIGINT handler only cancels a context, which a blocking
	// read cannot observe, so watch for the interrupt here to keep Ctrl+C working.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	type result struct {
		buf      []byte
		overflow bool
		err      error
	}

	resCh := make(chan result, 1)

	go func() {
		buf, overflow, err := bufio.NewReader(l.r).ReadLine()
		resCh <- result{buf, overflow, err}
	}()

	select {
	case <-sigCtx.Done():
		l.Println()
		return "", context.Canceled
	case res := <-resCh:
		if res.err != nil {
			return "", res.err
		}

		if res.overflow {
			return "", ErrInputOverflow{}
		}

		return string(res.buf), nil
	}
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
