package metadata

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

type HTTPRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func ReadCache(cachePath string) ([]byte, error) {
	fp, err := os.Open(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	defer fp.Close()

	s, err := io.ReadAll(fp)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Download the metadata for aur packages.
// create cache file
// write to cache file.
func MakeCache(ctx context.Context, httpClient HTTPRequestDoer, cachePath string) ([]byte, error) {
	body, err := downloadAURMetadata(ctx, httpClient)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	s, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(cachePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if _, err = f.Write(s); err != nil {
		return nil, err
	}

	return s, err
}

func downloadAURMetadata(ctx context.Context, httpClient HTTPRequestDoer) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://aur.archlinux.org/packages-meta-ext-v1.json.gz", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download metadata: %s", resp.Status)
	}

	return resp.Body, nil
}
