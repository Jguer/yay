package translations

import (
	"fmt"
	"os"
	r "reflect"
	"strings"
	"unicode"
)

// Translation represent a translation in a language
type Translation struct {
	Code   string
	values map[string]string
}

// Stub struct for grouping the Translation constructors together for reflection
type langauge struct{}

// findLang is the helper function to determine the langauge based on the
// envoirment
func findLang() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = os.Getenv("LC_ALL")
		if lang == "" {
			// TODO Filter with --verbose
			fmt.Printf("Could not determine the LANG based on envoirment, %s",
				"Falling back to english.")
			return "en_US"
		}
	}

	encodingDot := strings.IndexRune(lang, '.')
	if encodingDot > -1 {
		lang = lang[:encodingDot]
	}
	return lang
}

// GetTranslation gets the current translation struct based on envoirment
func GetTranslation() Translation {
	lang := findLang()
	// Removes the underscore in the ISO format because we can't create
	// methods with underscores
	lang = lang[:2] + lang[3:]
	// Method has to be public
	lang = string(unicode.ToUpper(rune(lang[0]))) + lang[1:]
	langMethod := r.ValueOf(langauge{}).MethodByName(lang)
	if langMethod.IsNil() {
		langMethod = r.ValueOf(langauge{}).MethodByName(lang[:2])
		if langMethod.IsNil() {
			langMethod = r.ValueOf(langauge{}).MethodByName("EnUS")
		}
	}
	translation := langMethod.Call([]r.Value{})[0].Interface().(Translation)
	return translation
}

// GetStr returns the value translated to the current loaded translation,
// automatically falls back to english if it cant find the translated key
// or default error string if key doesn't exists
func (t *Translation) GetStr(key string) string {
	val, ok := t.values[key]
	if !ok {
		val, ok = enUsMap[key]
		if !ok {
			// I think is better to always return something then to handle a
			// error everytime, but I not sure if we should keep going without
			// a string, a argument could be made to quit here.
			return "ERROR: invalid translation key!!!"
		}
	}
	return val
}
