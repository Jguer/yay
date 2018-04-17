package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	alpm "github.com/jguer/go-alpm"
)

func questionCallback(question alpm.QuestionAny) {
	qi, err := question.QuestionInstallIgnorepkg()
	if err == nil {
		qi.SetInstall(true)
	}

	qp, err := question.QuestionSelectProvider()
	if err == nil {
		size := 0

		qp.Providers(alpmHandle).ForEach(func(pkg alpm.Package) error {
			size++
			return nil
		})

		fmt.Print(bold(cyan(":: ")))
		str := bold(fmt.Sprintf(bold("There are %d providers available for %s:"), size, qp.Dep()))

		size = 1
		var db string

		qp.Providers(alpmHandle).ForEach(func(pkg alpm.Package) error {
			thisDb := pkg.DB().Name()

			if db != thisDb {
				db = thisDb
				str += bold(cyan("\n:: ")) + bold("Repository "+db+"\n    ")
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
				fmt.Println(err)
				break
			}

			if overflow {
				fmt.Println("Input too long")
				continue
			}

			if string(numberBuf) == "" {
				break
			}

			num, err := strconv.Atoi(string(numberBuf))
			if err != nil {
				fmt.Printf("%s invalid number: %s\n", red("error:"), string(numberBuf))
				continue
			}

			if num < 1 || num > size {
				fmt.Printf("%s invalid value: %d is not between %d and %d\n", red("error:"), num, 1, size)
				continue
			}

			qp.SetUseIndex(num - 1)
			break
		}
	}
}
