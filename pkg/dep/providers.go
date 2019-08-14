package dep

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
	rpc "github.com/mikkeloscar/aur"
)

type providers struct {
	lookfor string
	Pkgs    []*rpc.Pkg
}

func (q providers) Len() int {
	return len(q.Pkgs)
}

func (q providers) Less(i, j int) bool {
	if q.lookfor == q.Pkgs[i].Name {
		return true
	}

	if q.lookfor == q.Pkgs[j].Name {
		return false
	}

	return types.LessRunes([]rune(q.Pkgs[i].Name), []rune(q.Pkgs[j].Name))
}

func (q providers) Swap(i, j int) {
	q.Pkgs[i], q.Pkgs[j] = q.Pkgs[j], q.Pkgs[i]
}

func makeProviders(name string) providers {
	return providers{
		name,
		make([]*rpc.Pkg, 0),
	}
}

func providerMenu(dep string, providers providers, noconfirm bool) *rpc.Pkg {
	size := providers.Len()

	fmt.Print(text.Bold(text.Cyan(":: ")))
	str := text.Bold(fmt.Sprintf(text.Bold("There are %d providers available for %s:"), size, dep))

	size = 1
	str += text.Bold(text.Cyan("\n:: ")) + text.Bold("Repository AUR\n    ")

	for _, pkg := range providers.Pkgs {
		str += fmt.Sprintf("%d) %s ", size, pkg.Name)
		size++
	}

	fmt.Fprintln(os.Stderr, str)

	for {
		fmt.Print("\nEnter a number (default=1): ")

		if noconfirm {
			fmt.Println("1")
			return providers.Pkgs[0]
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
			return providers.Pkgs[0]
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s invalid number: %s\n", text.Red("error:"), string(numberBuf))
			continue
		}

		if num < 1 || num >= size {
			fmt.Fprintf(os.Stderr, "%s invalid value: %d is not between %d and %d\n", text.Red("error:"), num, 1, size-1)
			continue
		}

		return providers.Pkgs[num-1]
	}

	return nil
}
