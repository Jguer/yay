package runtime

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	alpm "github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/text"
)

const smallArrow = " ->"
const arrow = "==>"

// AlpmHandle keeps callback handle. TEMPORARY

func callbackQuestion(config *Configuration, alpmHandle *alpm.Handle, question alpm.QuestionAny) {
	if qi, err := question.QuestionInstallIgnorepkg(); err == nil {
		qi.SetInstall(true)
	}

	qp, err := question.QuestionSelectProvider()
	if err != nil {
		return
	}

	if config.HideMenus {
		return
	}

	size := 0

	if alpmHandle == nil {
		fmt.Println("alpmHandle is unset.")
		return
	}

	qp.Providers(alpmHandle).ForEach(func(pkg alpm.Package) error {
		size++
		return nil
	})

	fmt.Print(text.Bold(text.Cyan(":: ")))
	str := text.Bold(fmt.Sprintf(text.Bold("There are %d providers available for %s:"), size, qp.Dep()))

	size = 1
	var db string

	qp.Providers(alpmHandle).ForEach(func(pkg alpm.Package) error {
		thisDB := pkg.DB().Name()

		if db != thisDB {
			db = thisDB
			str += text.Bold(text.Cyan("\n:: ")) + text.Bold("Repository "+db+"\n    ")
		}
		str += fmt.Sprintf("%d) %s ", size, pkg.Name())
		size++
		return nil
	})

	fmt.Println(str)

	for {
		fmt.Print("\nEnter a number (default=1): ")

		if config.NoConfirm {
			fmt.Println()
			break
		}

		reader := bufio.NewReader(os.Stdin)
		numberBuf, overflow, err := reader.ReadLine()

		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}

		if overflow {
			fmt.Fprintln(os.Stderr, "Input too long")
			continue
		}

		if string(numberBuf) == "" {
			break
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s invalid number: %s\n", text.Red("error:"), string(numberBuf))
			continue
		}

		if num < 1 || num > size {
			fmt.Fprintf(os.Stderr, "%s invalid value: %d is not between %d and %d\n", text.Red("error:"), num, 1, size)
			continue
		}

		qp.SetUseIndex(num - 1)
		break
	}
}

func callbackLog(level alpm.LogLevel, str string) {
	switch level {
	case alpm.LogWarning:
		fmt.Print(text.Bold(text.Yellow(smallArrow)), " ", str)
	case alpm.LogError:
		fmt.Print(text.Bold(text.Red(smallArrow)), " ", str)
	}
}
