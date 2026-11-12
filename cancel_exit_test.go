package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"
)

func TestCommandExitCode(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 42")
	exitErr := cmd.Run()
	for _, tc := range []struct {
		name   string
		cancel bool
		err    error
		want   int
		quiet  bool
	}{
		{"success", false, nil, 0, true},
		{"error", false, errors.New("command failed"), 1, false},
		{"subprocess", false, exitErr, 42, false},
		{"cancelled input", true, fmt.Errorf("input: %w", context.Canceled), 130, true},
		{"cancelled confirmation", true, nil, 130, true},
		{"cancelled user abort", true, settings.ErrUserAbort{}, 130, true},
		{"cancelled subprocess", true, exitErr, 130, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			var output bytes.Buffer
			log := text.NewLogger(&output, &output, nil, false, "test")
			if got := commandExitCode(ctx, tc.err, log); got != tc.want {
				t.Fatalf("exit code = %d, want %d", got, tc.want)
			}
			if tc.quiet && output.Len() != 0 {
				t.Fatalf("unexpected error output: %s", &output)
			}
			if !tc.quiet && output.Len() == 0 {
				t.Fatal("missing command error output")
			}
		})
	}
}
