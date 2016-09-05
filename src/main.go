package main

import (
	"bufio"
	"flag"
	"fmt"
	c "github.com/fatih/color"
	"os"
	"strconv"
	"strings"
)

// PacmanBin describes the default installation point of pacman
const PacmanBin string = "/usr/bin/pacman"

// SearchMode is search without numbers
const SearchMode int = -1

func getNums() (numbers []int, err error) {
	var numberString string
	green := c.New(c.FgGreen).SprintFunc()

	fmt.Printf("%s\nNumbers:", green("Type numbers to install. Separate each number with a space."))
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

func defaultMode(pkg string) {
	aurRes := searchAurPackages(pkg)
	repoRes, err := SearchPackages(pkg)
	repoRes.printSearch(0)
	err = aurRes.printSearch(repoRes.Resultcount)
	nums, err := getNums()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(nums)

}

func main() {
	flag.Parse()
	searchTerm := flag.Args()
	defaultMode(searchTerm[0])
}
