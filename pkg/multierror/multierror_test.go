//go:build !integration
// +build !integration

package multierror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiError(t *testing.T) {
	t.Parallel()

	merr := &MultiError{}
	require.NoError(t, merr.Return())

	merr.Add(errors.New("first"))
	require.Equal(t, "first", merr.Error())
	require.EqualError(t, merr.Return(), "first")

	merr.Add(nil)
	merr.Add(errors.New("second"))
	require.Equal(t, "first\nsecond", merr.Error())
}
