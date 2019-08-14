package text

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const smallArrow = " ->"
const arrow = "==>"

// GetInput handles and treats user input
func GetInput(defaultValue string, noConfirm bool) (string, error) {
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
		return "", fmt.Errorf("Input too long")
	}

	return string(buf), nil
}

// ContinueTask prompts if user wants to continue task.
//If NoConfirm is set the action will continue without user input.
func ContinueTask(s string, cont bool, noConfirm bool) bool {
	if noConfirm {
		return cont
	}

	var response string
	var postFix string
	yes := "yes"
	no := "no"
	y := string([]rune(yes)[0])
	n := string([]rune(no)[0])

	if cont {
		postFix = fmt.Sprintf(" [%s/%s] ", strings.ToUpper(y), n)
	} else {
		postFix = fmt.Sprintf(" [%s/%s] ", y, strings.ToUpper(n))
	}

	fmt.Print(Bold(Green(arrow)+" "+s), Bold(postFix))

	if _, err := fmt.Scanln(&response); err != nil {
		return cont
	}

	response = strings.ToLower(response)
	return response == yes || response == y
}
