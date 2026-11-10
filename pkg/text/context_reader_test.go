package text

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type blockedInput struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
	read     atomic.Bool
}

func (r *blockedInput) Read(p []byte) (int, error) {
	if r.read.Swap(true) {
		return 0, io.EOF
	}
	close(r.started)
	<-r.release
	n := copy(p, "late input\n")
	close(r.finished)
	return n, nil
}

func TestInputCancellation(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		name := "selection"
		if confirm {
			name = "confirmation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := &blockedInput{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
			defer close(input.release)
			log := NewLogger(io.Discard, io.Discard, NewContextReader(ctx, input), false, "test").Child("child")
			result := make(chan error, 1)
			go func() {
				if confirm {
					if log.ContinueTask("Continue?", true, false) {
						result <- errors.New("cancelled confirmation accepted")
						return
					}
					result <- nil
					return
				}
				_, err := log.GetInput("", false)
				if !errors.Is(err, context.Canceled) {
					result <- errors.New("selection did not return context.Canceled")
					return
				}
				result <- nil
			}()
			select {
			case <-input.started:
			case <-time.After(time.Second):
				t.Fatal("input not read")
			}
			cancel()
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("prompt did not return on cancellation")
			}
		})
	}
}

func TestContextReaderReadOwnership(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := &blockedInput{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
	reader := NewContextReader(ctx, input)
	buf := []byte("unchanged!")
	done := make(chan error, 1)
	go func() { _, err := reader.Read(buf); done <- err }()
	<-input.started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		close(input.release)
		t.Fatal("read did not cancel")
	}
	close(input.release)
	<-input.finished
	if string(buf) != "unchanged!" {
		t.Fatalf("underlying read mutated caller buffer after return: %q", buf)
	}
	if _, err := reader.Read(buf); !errors.Is(err, context.Canceled) {
		t.Fatalf("read after cancel: %v", err)
	}
}

func TestPartialConfirmationCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := &blockedInput{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
	defer close(input.release)
	log := NewLogger(io.Discard, io.Discard, NewContextReader(ctx, io.MultiReader(strings.NewReader("y"), input)), false, "test")
	result := make(chan bool, 1)
	go func() { result <- log.ContinueTask("Continue?", true, false) }()
	select {
	case <-input.started:
	case <-time.After(time.Second):
		t.Fatal("partial input not read")
	}
	cancel()
	select {
	case accepted := <-result:
		if accepted {
			t.Fatal("partial confirmation accepted after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("partial confirmation did not cancel")
	}
}

func TestContextReaderCancelledBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input := &blockedInput{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
	defer close(input.release)
	reader := NewContextReader(ctx, input)
	for range 10 {
		if _, err := reader.Read(make([]byte, 4)); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	select {
	case <-input.started:
		t.Fatal("cancelled reader started underlying I/O")
	default:
	}
}

func TestContextReaderNormalInput(t *testing.T) {
	for _, cancellable := range []bool{false, true} {
		ctx := context.Background()
		if cancellable {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
		}
		log := NewLogger(io.Discard, io.Discard, NewContextReader(ctx, strings.NewReader("2 3\n")), false, "test")
		value, err := log.GetInput("", false)
		if err != nil || value != "2 3" {
			t.Fatalf("normal input: %q %v", value, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	log := NewLogger(io.Discard, io.Discard, NewContextReader(ctx, strings.NewReader("yes\n")), false, "test")
	if log.ContinueTask("Continue?", true, false) {
		t.Fatal("already-cancelled confirmation accepted")
	}
}
