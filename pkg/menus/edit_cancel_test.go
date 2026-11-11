package menus

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/stretchr/testify/require"
)

type cancelledEditorInput struct {
	t     *testing.T
	err   error
	reads int
}

func (r *cancelledEditorInput) Read([]byte) (int, error) {
	r.reads++
	if r.reads > 1 {
		r.t.Fatal("editor retried cancelled input")
	}
	return 0, fmt.Errorf("input: %w", r.err)
}

func TestEditPkgbuildsCancelledInput(t *testing.T) {
	t.Setenv("EDITOR", "")
	t.Setenv("VISUAL", "")
	for _, want := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(want.Error(), func(t *testing.T) {
			input := &cancelledEditorInput{t: t, err: want}
			var output bytes.Buffer
			log := text.NewLogger(&output, &output, input, false, "test")
			err := editPkgbuilds(log, map[string]string{"pkg": t.TempDir()}, []string{"pkg"}, "", "", nil, false)
			require.ErrorIs(t, err, want)
			require.Equal(t, 1, input.reads)
			require.NotContains(t, output.String(), want.Error())
		})
	}
}
