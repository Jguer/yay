package menus

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// runPager pages content through a single pager process. Overridable in tests.
var runPager = pageThroughPager

// isStdoutTerminal reports whether stdout is a terminal. Overridable in tests.
var isStdoutTerminal = func() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// resolvePager returns the pager command string.
// Priority: config pager → PAGER → less → cat.
func resolvePager(pagerConfig string) string {
	if pagerConfig != "" {
		return pagerConfig
	}

	if pager := os.Getenv("PAGER"); pager != "" {
		return pager
	}

	if _, err := exec.LookPath("less"); err == nil {
		return "less"
	}

	return "cat"
}

// withLessRawANSI adds -R when the pager is less. A command-line option
// overrides both $LESS and a lesskey file, so colored diffs render even when
// the user's less configuration leaves -R out.
func withLessRawANSI(pager string) string {
	fields := strings.Fields(pager)
	if len(fields) > 0 && filepath.Base(fields[0]) == "less" {
		return pager + " -R"
	}

	return pager
}

// pageThroughPager runs the configured pager with content on stdin.
// The pager command is user-controlled (config / $PAGER), same model as $EDITOR.
func pageThroughPager(ctx context.Context, content, pagerConfig string) error {
	pager := withLessRawANSI(resolvePager(pagerConfig))
	cmd := exec.CommandContext(ctx, "sh", "-c", pager)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if os.Getenv("LESS") == "" {
		// S: chop long lines; R: raw ANSI; X: no termcap init; F: quit if one screen
		cmd.Env = append(cmd.Env, "LESS=SRXF")
	}

	return cmd.Run()
}
