package main

import (
	"bufio"
	"errors"
	"flag"
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

func installnumArray(num []int, aurRes AurSearch, repoRes RepoSearch) (err error) {
	if len(num) == 0 {
		return errors.New("Installing AUR array: No nums selected")
	}

	var index int
	for _, i := range num {
		if i > repoRes.Resultcount-1 {
			index = i - repoRes.Resultcount
			fmt.Printf("%+v\n\n", aurRes.Results[i-index])
			err = aurRes.Results[i-index].installResult()
			if err != nil {
				// Do not abandon program, we might still be able to install the rest
				fmt.Println(err)
			}
		} else {

		}
	}

	return err
}

func defaultMode(pkg string) (err error) {
	aurRes, err := searchAurPackages(pkg)
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

func main() {
	flag.Parse()
	if os.Getenv("EDITOR") != "" {
		Editor = os.Getenv("EDITOR")
	}
	searchTerm := flag.Args()
	err := defaultMode(searchTerm[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
