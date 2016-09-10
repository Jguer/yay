package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PacmanBin describes the default installation point of pacman
const PacmanBin string = "/usr/bin/pacman"

// MakepkgBin describes the default installation point of makepkg command
const MakepkgBin string = "/usr/bin/makepkg"

// TarBin describes the default installation point of tar command
// Probably will replace untar with code solution
const TarBin string = "/usr/bin/tar"

// SearchMode is search without numbers
const SearchMode int = -1

// BuildDir is the root for package building
const BuildDir string = "/tmp/yaytmp/"

// BaseURL givers the AUR default address
const BaseURL string = "https://aur.archlinux.org"

// Editor gives the default system editor, uses vi in last case
var Editor = "vi"

func getNums() (numbers []int, err error) {
	var numberString string
	fmt.Printf("\x1B[32m%s\033[0m\nNumbers:", "Type numbers to install. Separate each number with a space.")
	reader := bufio.NewReader(os.Stdin)
	numberString, err = reader.ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	result := strings.Fields(numberString)
	var num int
	for _, numS := range result {
		num, err = strconv.Atoi(numS)
		if err != nil {
			fmt.Println(err)
			return
		}
		numbers = append(numbers, num)
	}

	return
}

func defaultMode(pkg string) (err error) {
	aurRes, err := searchAurPackages(pkg, 0)
	repoRes, err := SearchPackages(pkg)
	if err != nil {
		return
	}

	if repoRes.Resultcount == 0 && aurRes.Resultcount == 0 {
		return errors.New("No Packages match search")
	}
	repoRes.printSearch(0)
	err = aurRes.printSearch(repoRes.Resultcount)

	nums, err := getNums()
	if err != nil {
		return
	}
	err = installnumArray(nums, aurRes, repoRes)
	if err != nil {
		return
	}
	return
}

func searchMode(pkg string) (err error) {
	aur, err := searchAurPackages(pkg, SearchMode)
	repo, err := SearchPackages(pkg)
	if err != nil {
		return err
	}

	aur.printSearch(SearchMode)
	repo.printSearch(SearchMode)

	return nil
}

func main() {
	var err error
	if os.Getenv("EDITOR") != "" {
		Editor = os.Getenv("EDITOR")
	}
	if os.Args[1] == "-Ss" {
		err = searchMode(strings.Join(os.Args[3:], " "))

	} else if os.Args[1] == "-S" {
		err = InstallPackage(os.Args[2], os.Args[3:]...)
	} else {
		err = defaultMode(os.Args[1])
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
