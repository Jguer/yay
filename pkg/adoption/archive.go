package adoption

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const ExtArchiveURL = "https://aur.archlinux.org/packages-meta-ext-v1.json.gz"

type FetchResult struct {
	Entries      map[string]Entry
	ETag         string
	LastModified time.Time
	NotModified  bool
}

// Fetch the archive & pull relevant info from it when GET is not 304
func Fetch(ctx context.Context, client *http.Client, prevETag string, prevLastModified time.Time) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ExtArchiveURL, nil)
	if err != nil {
		return FetchResult{}, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}
	if !prevLastModified.IsZero() {
		req.Header.Set("If-Modified-Since", prevLastModified.UTC().Format(http.TimeFormat))
	}

	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return FetchResult{NotModified: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return FetchResult{}, fmt.Errorf("archive fetch: unexpected status %s", resp.Status)
	}

	// Explicit header kills net/http. Use gzip instead.
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("archive gzip: %w", err)
	}
	defer zr.Close()

	entries, err := parseEntries(zr)
	if err != nil {
		return FetchResult{}, fmt.Errorf("archive parse: %w", err)
	}

	lastModified, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return FetchResult{
		Entries:      entries,
		ETag:         resp.Header.Get("ETag"),
		LastModified: lastModified,
	}, nil
}

func parseEntries(r io.Reader) (map[string]Entry, error) {
	dec := json.NewDecoder(r)
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil, fmt.Errorf("unexpected top-level token %v", tok)
	}

	out := make(map[string]Entry, 1<<16)
	consumeArray := func() error {
		for dec.More() {
			var p struct {
				Name         string `json:"Name"`
				Maintainer   string `json:"Maintainer"`
				LastModified int64  `json:"LastModified"`
			}
			if err := dec.Decode(&p); err != nil {
				return err
			}
			if p.Name != "" {
				out[p.Name] = Entry{Maintainer: p.Maintainer, LastModified: p.LastModified}
			}
		}
		_, err := dec.Token() // closing ']'
		return err
	}

	switch delim {
	case '[':
		if err := consumeArray(); err != nil {
			return nil, err
		}
	case '{':
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, _ := keyTok.(string)
			if key == "results" {
				t2, err := dec.Token()
				if err != nil {
					return nil, err
				}
				if d, ok := t2.(json.Delim); !ok || d != '[' {
					return nil, fmt.Errorf("results is not an array")
				}
				if err := consumeArray(); err != nil {
					return nil, err
				}
			} else if err := skipValue(dec); err != nil {
				return nil, err
			}
		}
		if _, err := dec.Token(); err != nil { // closing '}'
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delim)
	}
	return out, nil
}

func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil // scalar consumed
	}
	for dec.More() {
		if d == '{' {
			if _, err := dec.Token(); err != nil { // key
				return err
			}
		}
		if err := skipValue(dec); err != nil { // value or element
			return err
		}
	}
	_, err = dec.Token() // closing delim
	return err
}
