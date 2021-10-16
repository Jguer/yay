package text

import (
	"bufio"
	"fmt"
	"os"
)

func GetInput(defaultValue string, noConfirm bool) (string, error) {
	Info()

	if defaultValue != "" || noConfirm {
		fmt.Println(defaultValue)
		return defaultValue, nil
	}

	reader := bufio.NewReader(os.Stdin)

	buf, overflow, err := reader.ReadLine()
	if err != nil {
		return "", err
	}

	if overflow {
		return "", ErrInputOverflow{}
	}

	return string(buf), nil
}
