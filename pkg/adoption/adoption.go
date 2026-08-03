// Package adoption tracks recent AUR maintainer changes by diffing periodic
// snapshots of the bulk metadata archive, and reports them as warnings.
//
// NOTE: This is a "workaround," the proper fix is to have the AUR RPC
// handle maintainership data.
//
// Caches package modified time and maintainer, updates on refresh & diffs
// against the previous capture to flag changes. 
//
// Reconstruction has one inherent gap: packages absent from the prior snapshot
// carry no baseline, so their pre-baseline history cannot be dated. This is the
// consequence of the AUR not timestamping maintainer changes server-side.
package adoption

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StoreFileName is the on-disk name of the persisted store, placed in the cache
// directory by the caller.
const StoreFileName = "adoption.json"

const (
	// Alert the user if the maintainer has changed within the last 30 days. 
	// Change as you see fit.
	Window = 30 * 24 * time.Hour
	// Tolerate a maintainership cache that's no older than 24h before 
	// refreshing on next search / transaction. Change as you see fit.
	DefaultMaxAge = 24 * time.Hour
)

var NowFunc = time.Now

type Kind string

const (
	// Flags changing from one value to another, be that maintained > orphan
	// or maintainer A > maintainer B.
	KindMaintainerChange Kind = "maintainer-change"
	// Covers if a malactor adopts a package, poisons it, and orphans it immediately.
	// I encourage further Kind additions of any are thought of. 
	KindOrphanModified Kind = "orphan-modified"
)

type Entry struct {
	Maintainer   string `json:"m"`
	LastModified int64  `json:"lm"`
}

type Snapshot struct {
	ETag         string           `json:"etag"`
	LastModified time.Time        `json:"last_modified"` // archive Last-Modified header
	FetchedAt    time.Time        `json:"fetched_at"`
	Entries      map[string]Entry `json:"entries"`
}

type Record struct {
	Kind       Kind      `json:"kind"`
	From       string    `json:"from"`
	To         string    `json:"to"`
	ObservedAt time.Time `json:"observed_at"`
}

type Store struct {
	Snapshot Snapshot          `json:"snapshot"`
	Records  map[string]Record `json:"records"`

	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{
		path:     path,
		Records:  map[string]Record{},
		Snapshot: Snapshot{Entries: map[string]Entry{}},
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	if s.Records == nil {
		s.Records = map[string]Record{}
	}
	if s.Snapshot.Entries == nil {
		s.Snapshot.Entries = map[string]Entry{}
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Validators() (etag string, lastModified time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Snapshot.ETag, s.Snapshot.LastModified
}

func (s *Store) ShouldRefresh(maxAge time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return NowFunc().Sub(s.Snapshot.FetchedAt) >= maxAge
}

// Apply installs a freshly fetched snapshot, reconstructs transitions against
// the prior snapshot, records them anchored to observedAt (archive
// Last-Modified), prunes expired records.
func (s *Store) Apply(entries map[string]Entry, etag string, archiveLastModified, fetchedAt, observedAt time.Time) (map[string]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	changes := diff(s.Snapshot.Entries, entries, observedAt)
	for name, r := range changes {
		s.Records[name] = r
	}
	s.Snapshot = Snapshot{
		ETag:         etag,
		LastModified: archiveLastModified,
		FetchedAt:    fetchedAt,
		Entries:      entries,
	}
	s.pruneLocked()
	return changes, s.saveLocked()
}

func (s *Store) Touch(fetchedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Snapshot.FetchedAt = fetchedAt
	s.pruneLocked()
	return s.saveLocked()
}

// Check single package upon transaction (ex: install), if a transition is found, it's returned.
func (s *Store) Check(name, maintainer string, lastModified int64, observedAt time.Time) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur := Entry{Maintainer: maintainer, LastModified: lastModified}
	prev, seen := s.Snapshot.Entries[name]
	s.Snapshot.Entries[name] = cur
	if !seen {
		return Record{}, false
	}
	r, ok := transition(prev, cur, observedAt)
	if ok {
		s.Records[name] = r
	}
	_ = s.saveLocked()
	return r, ok
}

// Current records; prune expired.
func (s *Store) Active() map[string]Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	out := make(map[string]Record, len(s.Records))
	for k, v := range s.Records {
		out[k] = v
	}
	return out
}

func (s *Store) pruneLocked() {
	cutoff := NowFunc().Add(-Window)
	for name, r := range s.Records {
		if r.ObservedAt.Before(cutoff) {
			delete(s.Records, name)
		}
	}
}

func diff(prev, cur map[string]Entry, observedAt time.Time) map[string]Record {
	out := make(map[string]Record)
	for name, c := range cur {
		p, seen := prev[name]
		if !seen {
			continue
		}
		if r, ok := transition(p, c, observedAt); ok {
			out[name] = r
		}
	}
	return out
}

// We do what we can.
func transition(prev, cur Entry, observedAt time.Time) (Record, bool) {
	switch {
	case prev.Maintainer != cur.Maintainer:
		return Record{KindMaintainerChange, prev.Maintainer, cur.Maintainer, observedAt}, true
	case prev.Maintainer == "" && cur.Maintainer == "" && cur.LastModified > prev.LastModified:
		return Record{KindOrphanModified, "", "", observedAt}, true
	}
	return Record{}, false
}
