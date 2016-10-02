package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/aur"
	"os"
	"strconv"
	"strings"
)

func searchAndInstall(pkgName string, conf *alpm.PacmanConfig, flags string) (err error) {
	var num int
	var numberString string
	var pacBuffer bytes.Buffer

	a, err := aur.Search(pkgName, true)
	r, err := SearchPackages(pkgName, conf)
	if err != nil {
		return
	}

	if len(r.Results) == 0 && a.Resultcount == 0 {
		return errors.New("No Packages match search")
	}
	r.PrintSearch(0)
	a.PrintSearch(len(r.Results))

	fmt.Printf("\x1B[32m%s\033[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberString, err = reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	var aurInstall []aur.Result
	result := strings.Fields(numberString)
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// Install package
		if num > len(r.Results)-1 {
			aurInstall = append(aurInstall, a.Results[num-len(r.Results)])
		} else {
			pacBuffer.WriteString(r.Results[num].Name)
			pacBuffer.WriteString(" ")
		}
	}

	InstallPackage(strings.TrimSpace(pacBuffer.String()), conf, flags)

	for _, aurpkg := range aurInstall {
		err = aurpkg.Install(BuildDir, conf, flags)
		if err != nil {
			// Do not abandon program, we might still be able to install the rest
			fmt.Println(err)
		}
	}
	return
}

// updateAndInstall handles updating the cache and installing updates
func updateAndInstall(conf *alpm.PacmanConfig, flags string) error {
	errp := UpdatePackages(flags)
	erra := aur.UpdatePackages(BuildDir, conf, flags)

	if errp != nil {
		return errp
	}

	return erra
}

func searchMode(pkg string, conf *alpm.PacmanConfig) (err error) {
	a, err := aur.Search(pkg, true)
	if err != nil {
		return err
	}
	r, err := SearchPackages(pkg, conf)
	if err != nil {
		return err
	}

	r.PrintSearch(SearchMode)
	a.PrintSearch(SearchMode)

	return nil
}
