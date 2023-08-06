package text

import (
	"bufio"
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
