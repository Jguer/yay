package text

import (
	"bufio"
	"fmt"
	"os"

	"github.com/leonelquinteros/gotext"
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
		return "", fmt.Errorf(gotext.Get("input too long"))
	}

	return string(buf), nil
}
