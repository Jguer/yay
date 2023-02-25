package text

import (
	"bufio"
	"fmt"
	"io"
)

func (l *Logger) GetInput(defaultValue string, noConfirm bool) (string, error) {
	Info()

	if defaultValue != "" || noConfirm {
		fmt.Println(defaultValue)
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

func GetInput(r io.Reader, defaultValue string, noConfirm bool) (string, error) {
	return GlobalLogger.GetInput(defaultValue, noConfirm)
}
