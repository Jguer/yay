package aur

import (
	"encoding/json"
	"net/http"
	"os"
)

// BaseURL givers the AUR default address.
const BaseURL string = "https://aur.archlinux.org"

// Editor gives the default system editor, uses vi in last case
var Editor = "vi"

func init() {
	if os.Getenv("EDITOR") != "" {
		Editor = os.Getenv("EDITOR")
	}
}

// getJSON handles JSON retrieval and decoding to struct
func getJSON(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}
