package adoption

import (
	"context"
	"net/http"
	"time"
)

func Refresh(ctx context.Context, client *http.Client, store *Store, maxAge time.Duration, force bool) (int, error) {
	if !force && !store.ShouldRefresh(maxAge) {
		return 0, nil
	}

	etag, lastModified := store.Validators()
	res, err := Fetch(ctx, client, etag, lastModified)
	if err != nil {
		return 0, err
	}

	now := NowFunc()
	if res.NotModified {
		return 0, store.Touch(now)
	}

	observedAt := res.LastModified
	if observedAt.IsZero() {
		observedAt = now
	}
	changes, err := store.Apply(res.Entries, res.ETag, res.LastModified, now, observedAt)
	return len(changes), err
}
