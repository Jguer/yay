package exec

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Jguer/yay/v10/pkg/text"
)

// Editor returns the preferred system editor with the extra arguments needed
func Editor(editorBin string, editorFlags string, noConfirm bool) (string, []string) {
	switch {
	case editorBin != "":
		editor, err := exec.LookPath(editorBin)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, strings.Fields(editorFlags)
		}
		fallthrough
	case os.Getenv("EDITOR") != "":
		editorArgs := strings.Fields(os.Getenv("EDITOR"))
		editor, err := exec.LookPath(editorArgs[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, editorArgs[1:]
		}
		fallthrough
	case os.Getenv("VISUAL") != "":
		editorArgs := strings.Fields(os.Getenv("VISUAL"))
		editor, err := exec.LookPath(editorArgs[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			return editor, editorArgs[1:]
		}
		fallthrough
	default:
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, text.Bold(text.Red(arrow)), text.Bold(text.Cyan("$EDITOR")), text.Bold("is not set"))
		fmt.Fprintln(os.Stderr, text.Bold(text.Red(arrow))+text.Bold(" Please add ")+text.Bold(text.Cyan("$EDITOR"))+text.Bold(" or ")+text.Bold(text.Cyan("$VISUAL"))+text.Bold(" to your environment variables."))

		for {
			fmt.Print(text.Green(text.Bold(arrow + " Edit PKGBUILD with: ")))
			editorInput, err := text.GetInput("", noConfirm)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			editorArgs := strings.Fields(editorInput)

			editor, err := exec.LookPath(editorArgs[0])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			return editor, editorArgs[1:]
		}
	}
}
