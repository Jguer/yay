package metadata

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	// check if cache exists
	cachePath := "aur.json"
	cacheBytes, err := ReadCache(cachePath)
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(cacheBytes) == 0 {
		cacheBytes, err = MakeCache(cachePath)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func MakeOrReadCache(cachePath string) ([]byte, error) {
	cacheBytes, err := ReadCache(cachePath)
	if err != nil {
		return nil, err
	}

	if len(cacheBytes) == 0 {
		cacheBytes, err = MakeCache(cachePath)
		if err != nil {
			return nil, err
		}
	}

	return cacheBytes, nil
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
// write to cache file
func MakeCache(cachePath string) ([]byte, error) {
	body, err := downloadAURMetadata()
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

func downloadAURMetadata() (io.ReadCloser, error) {
	resp, err := http.Get("https://aur.archlinux.org/packages-meta-ext-v1.json.gz")
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download metadata: %s", resp.Status)
	}

	return resp.Body, nil
}
