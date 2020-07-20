package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	alpm "github.com/Jguer/go-alpm"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

func questionCallback(question alpm.QuestionAny) {
	if qi, err := question.QuestionInstallIgnorepkg(); err == nil {
		qi.SetInstall(true)
	}

	qp, err := question.QuestionSelectProvider()
	if err != nil {
		return
	}

	if settings.HideMenus {
		return
	}

	size := 0

	_ = qp.Providers(config.Runtime.AlpmHandle).ForEach(func(pkg alpm.Package) error {
		size++
		return nil
	})

	str := text.Bold(gotext.Get("There are %d providers available for %s:\n", size, qp.Dep()))

	size = 1
	var db string

	_ = qp.Providers(config.Runtime.AlpmHandle).ForEach(func(pkg alpm.Package) error {
		thisDB := pkg.DB().Name()

		if db != thisDB {
			db = thisDB
			str += text.SprintOperationInfo(gotext.Get("Repository"), db, "\n    ")
		}
		str += fmt.Sprintf("%d) %s ", size, pkg.Name())
		size++
		return nil
	})

	text.OperationInfoln(str)

	for {
		fmt.Print(gotext.Get("\nEnter a number (default=1): "))

		if config.NoConfirm {
			fmt.Println()
			break
		}

		reader := bufio.NewReader(os.Stdin)
		numberBuf, overflow, err := reader.ReadLine()
		if err != nil {
			text.Errorln(err)
			break
		}

		if overflow {
			text.Errorln(gotext.Get(" Input too long"))
			continue
		}

		if string(numberBuf) == "" {
			break
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			text.Errorln(gotext.Get("invalid number: %s", string(numberBuf)))
			continue
		}

		if num < 1 || num > size {
			text.Errorln(gotext.Get("invalid value: %d is not between %d and %d", num, 1, size))
			continue
		}

		qp.SetUseIndex(num - 1)
		break
	}
}

func logCallback(level alpm.LogLevel, str string) {
	switch level {
	case alpm.LogWarning:
		text.Warn(str)
	case alpm.LogError:
		text.Error(str)
	}
}
