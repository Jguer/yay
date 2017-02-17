package aur

import (
	"encoding/json"
	"net/http"
)

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// getJSON handles JSON retrieval and decoding to struct
func getJSON(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}
