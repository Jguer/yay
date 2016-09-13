package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/aur"
	"os"
	"strconv"
	"strings"
)

func searchAndInstall(pkgName string, conf alpm.PacmanConfig, flags ...string) (err error) {
	var num int
	var numberString string

	aurRes, err := aur.Search(pkgName, true)
	repoRes, err := SearchPackages(pkgName, conf)
	if err != nil {
		return
	}

	if repoRes.Resultcount == 0 && aurRes.Resultcount == 0 {
		return errors.New("No Packages match search")
	}
	repoRes.printSearch(0)
	aurRes.PrintSearch(repoRes.Resultcount)

	fmt.Printf("\x1B[32m%s\033[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberString, err = reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	var index int
	result := strings.Fields(numberString)
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println(num)

		// Install package
		if num > repoRes.Resultcount-1 {
			index = num - repoRes.Resultcount
			err = aurRes.Results[num-index].Install(BuildDir, conf, flags...)
			if err != nil {
				// Do not abandon program, we might still be able to install the rest
				fmt.Println(err)
			}
		} else {
			InstallPackage(repoRes.Results[num].Name, conf, flags...)
		}
	}

	return
}

func searchMode(pkg string, conf alpm.PacmanConfig) (err error) {
	_, err = aur.Search(pkg, true)
	if err != nil {
		return err
	}
	repo, err := SearchPackages(pkg, conf)
	if err != nil {
		return err
	}

	aur.printSearch(SearchMode)
	repo.printSearch(SearchMode)

	return nil
}
