package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Jguer/yay/v13/pkg/text"
)

type promptReadyReader struct {
	input io.Reader
	ready bool
}

func (r *promptReadyReader) Read(p []byte) (int, error) {
	if !r.ready {
		fmt.Println("PROMPT_READY")
		r.ready = true
	}
	return r.input.Read(p)
}

func TestInterruptPromptProcess(t *testing.T) {
	if mode := os.Getenv("YAY_TEST_INTERRUPT_PROMPT"); mode != "" {
		ctx, stop := newSignalContext()
		defer stop()
		defer fmt.Println("CLEANUP_RAN")
		input := text.NewContextReader(ctx, &promptReadyReader{input: os.Stdin})
		log := text.NewLogger(io.Discard, io.Discard, input, false, "test").Child("prompt")
		if mode == "confirmation" {
			if log.ContinueTask("Continue?", true, false) {
				t.Fatal("cancelled prompt accepted")
			}
		} else {
			if _, err := log.GetInput("", false); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled input: %v", err)
			}
		}
		return
	}
	for _, mode := range []string{"selection", "confirmation"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInterruptPromptProcess$")
			cmd.Env = append(os.Environ(), "YAY_TEST_INTERRUPT_PROMPT="+mode)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			reader := bufio.NewReader(stdout)
			line, err := reader.ReadString('\n')
			if err != nil || line != "PROMPT_READY\n" {
				t.Fatalf("prompt not ready: %q %v", line, err)
			}
			if err = cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if err = cmd.Wait(); err != nil {
				t.Fatalf("prompt failed: %v %s", err, output)
			}
			if !strings.Contains(string(output), "CLEANUP_RAN") {
				t.Fatalf("deferred cleanup not run: %s", output)
			}
		})
	}
}
