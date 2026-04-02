//go:build !integration
// +build !integration

package sync

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Jguer/yay/v12/pkg/completion"
	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/stretchr/testify/require"
)

const waitGracePeriod = 50 * time.Millisecond

func TestStartCompletionUpdateSkipsWhenCacheIsFresh(t *testing.T) {
	originalNeedsUpdate := completionNeedsUpdate
	originalUpdateCache := completionUpdateCache
	t.Cleanup(func() {
		completionNeedsUpdate = originalNeedsUpdate
		completionUpdateCache = originalUpdateCache
	})

	updateCalled := false
	completionNeedsUpdate = func(string, int, bool) bool { return false }
	completionUpdateCache = func(context.Context, download.HTTPRequestDoer, completion.PkgSynchronizer,
		string, string, *text.Logger,
	) error {
		updateCalled = true
		return nil
	}

	service, run := newTestOperationService()

	wait := service.startCompletionUpdate(context.Background(), run)

	require.Nil(t, wait)
	require.False(t, updateCalled)
}

func TestStartCompletionUpdateWaitsForBackgroundUpdate(t *testing.T) {
	originalNeedsUpdate := completionNeedsUpdate
	originalUpdateCache := completionUpdateCache
	t.Cleanup(func() {
		completionNeedsUpdate = originalNeedsUpdate
		completionUpdateCache = originalUpdateCache
	})

	started := make(chan struct{})
	release := make(chan struct{})

	completionNeedsUpdate = func(string, int, bool) bool { return true }
	completionUpdateCache = func(context.Context, download.HTTPRequestDoer, completion.PkgSynchronizer,
		string, string, *text.Logger,
	) error {
		close(started)
		<-release
		return nil
	}

	service, run := newTestOperationService()

	wait := service.startCompletionUpdate(context.Background(), run)
	require.NotNil(t, wait)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("completion update did not start")
	}

	waitReturned := make(chan struct{})
	go func() {
		wait()
		close(waitReturned)
	}()

	select {
	case <-waitReturned:
		t.Fatal("wait returned before completion update finished")
	case <-time.After(waitGracePeriod):
	}

	close(release)

	select {
	case <-waitReturned:
	case <-time.After(time.Second):
		t.Fatal("wait did not return after completion update finished")
	}
}

func newTestOperationService() (*OperationService, *runtime.Runtime) {
	cfg := &settings.Configuration{
		AURURL:             "https://aur.archlinux.org",
		CompletionPath:     "/tmp/completion",
		CompletionInterval: 7,
	}
	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	run := &runtime.Runtime{
		Cfg:        cfg,
		HTTPClient: &http.Client{},
		Logger:     logger,
	}

	return NewOperationService(context.Background(), nil, run), run
}
