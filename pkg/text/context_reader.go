package text

import (
	"context"
	"io"
)

type contextReader struct {
	ctx   context.Context
	input io.Reader
}

// NewContextReader lets an interactive prompt return when ctx is cancelled.
// The underlying read may remain blocked until input arrives or the process
// exits. Cancellation is terminal: subsequent reads return ctx.Err() without
// starting another underlying read. It uses a private buffer so it cannot modify
// the caller's buffer after cancellation. Like the input reader, it is intended
// for sequential reads.
func NewContextReader(ctx context.Context, input io.Reader) io.Reader {
	if ctx.Done() == nil {
		return input
	}
	return &contextReader{ctx: ctx, input: input}
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		buf := make([]byte, len(p))
		n, err := r.input.Read(buf)
		done <- result{data: buf[:n], err: err}
	}()
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case res := <-done:
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		return copy(p, res.data), res.err
	}
}
