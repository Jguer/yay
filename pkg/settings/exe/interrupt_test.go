//go:build !integration

package exe

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

func TestBuildPacmanCmdCancelsLockWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	lock := filepath.Join(dir, "db.lck")
	if err := os.WriteFile(lock, []byte("another process"), 0600); err != nil {
		t.Fatal(err)
	}
	builder := &CmdBuilder{PacmanBin: "true", PacmanDBPath: dir, SudoBin: "true", Log: text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")}
	args := parser.MakeArguments()
	args.AddArg("S")
	args.AddTarget("test-package")
	done := make(chan error, 1)
	go func() { cmd := builder.BuildPacmanCmd(ctx, args, parser.ModeAny, false); done <- cmd.Run() }()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled command was executed")
		}
	case <-time.After(time.Second):
		// Unblock the old implementation without touching the real pacman lock.
		os.Remove(lock)
		select {
		case <-done:
		case <-time.After(4 * time.Second):
		}
		t.Fatal("lock wait did not observe cancellation")
	}
	data, err := os.ReadFile(lock)
	if err != nil || string(data) != "another process" {
		t.Fatalf("another process's lock changed: %q %v", data, err)
	}
}
