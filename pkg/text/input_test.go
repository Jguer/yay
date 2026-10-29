package text

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInputInterrupt(t *testing.T) {
	pr, pw := io.Pipe()
	defer pw.Close()

	l := NewLogger(io.Discard, io.Discard, pr, false, "test")

	done := make(chan error, 1)

	go func() {
		_, err := l.GetInput("", false)
		done <- err
	}()

	time.Sleep(100 * time.Millisecond)

	process, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, process.Signal(os.Interrupt))

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("GetInput did not return after interrupt")
	}
}

func TestGetInputReadsLine(t *testing.T) {
	l := NewLogger(io.Discard, io.Discard, strings.NewReader("1 2\n"), false, "test")

	got, err := l.GetInput("", false)
	require.NoError(t, err)
	assert.Equal(t, "1 2", got)
}
