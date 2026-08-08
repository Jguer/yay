package main

import (
	"os"
	"testing"
	"time"
)

func TestInterruptCancelsContext(t *testing.T) {
	ctx, stop := newSignalContext()
	defer stop()

	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("interrupt did not cancel context")
	}
}
