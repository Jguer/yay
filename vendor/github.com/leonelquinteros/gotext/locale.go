/*
 * Copyright (c) 2018 DeineAgentur UG https://www.deineagentur.com. All rights reserved.
 * Licensed under the MIT License. See LICENSE file in the project root for full license information.
 */

package gotext

import (
	"os"
	"path"
	"sync"
)

/*
Locale wraps the entire i18n collection for a single language (locale)
It's used by the package functions, but it can also be used independently to handle
multiple languages at the same time by working with this object.

Example:

    import (
	    "fmt"
	    "github.com/leonelquinteros/gotext"
    )

    func main() {
        // Create Locale with library path and language code
        l := gotext.NewLocale("/path/to/i18n/dir", "en_US")

        // Load domain '/path/to/i18n/dir/en_US/LC_MESSAGES/default.{po,mo}'
        l.AddDomain("default")

        // Translate text from default domain
        fmt.Println(l.Get("Translate this"))

        // Load different domain ('/path/to/i18n/dir/en_US/LC_MESSAGES/extras.{po,mo}')
        l.AddDomain("extras")

        // Translate text from domain
        fmt.Println(l.GetD("extras", "Translate this"))
    }

*/
type Locale struct {
	// Path to locale files.
	path string

	// Language for this Locale
	lang string

	// List of available Domains for this locale.
	Domains map[string]Translator

	// First AddDomain is default Domain
	defaultDomain string

	// Sync Mutex
	sync.RWMutex
}

// NewLocale creates and initializes a new Locale object for a given language.
// It receives a path for the i18n files directory (p) and a language code to use (l).
func NewLocale(p, l string) *Locale {
	return &Locale{
		path:    p,
		lang:    SimplifiedLocale(l),
		Domains: make(map[string]Translator),
	}
}

func (l *Locale) findExt(dom, ext string) string {
	filename := path.Join(l.path, l.lang, "LC_MESSAGES", dom+"."+ext)
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	if len(l.lang) > 2 {
		filename = path.Join(l.path, l.lang[:2], "LC_MESSAGES", dom+"."+ext)
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}

	filename = path.Join(l.path, l.lang, dom+"."+ext)
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	if len(l.lang) > 2 {
		filename = path.Join(l.path, l.lang[:2], dom+"."+ext)
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}

	return ""
}

// AddDomain creates a new domain for a given locale object and initializes the Po object.
// If the domain exists, it gets reloaded.
func (l *Locale) AddDomain(dom string) {
	var poObj Translator

	file := l.findExt(dom, "po")
	if file != "" {
		poObj = new(Po)
		// Parse file.
		poObj.ParseFile(file)
	} else {
		file = l.findExt(dom, "mo")
		if file != "" {
			poObj = new(Mo)
			// Parse file.
			poObj.ParseFile(file)
		} else {
			// fallback return if no file found with
			return
		}
	}

	// Save new domain
	l.Lock()

	if l.Domains == nil {
		l.Domains = make(map[string]Translator)
	}
	if l.defaultDomain == "" {
		l.defaultDomain = dom
	}
	l.Domains[dom] = poObj

	// Unlock "Save new domain"
	l.Unlock()
}

// GetDomain is the domain getter for the package configuration
func (l *Locale) GetDomain() string {
	l.RLock()
	dom := l.defaultDomain
	l.RUnlock()
	return dom
}

// SetDomain sets the name for the domain to be used at package level.
// It reloads the corresponding Translation file.
func (l *Locale) SetDomain(dom string) {
	l.Lock()
	l.defaultDomain = dom
	l.Unlock()
}

// Get uses a domain "default" to return the corresponding Translation of a given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) Get(str string, vars ...interface{}) string {
	return l.GetD(l.GetDomain(), str, vars...)
}

// GetN retrieves the (N)th plural form of Translation for the given string in the "default" domain.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetN(str, plural string, n int, vars ...interface{}) string {
	return l.GetND(l.GetDomain(), str, plural, n, vars...)
}

// GetD returns the corresponding Translation in the given domain for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetD(dom, str string, vars ...interface{}) string {
	return l.GetND(dom, str, str, 1, vars...)
}

// GetND retrieves the (N)th plural form of Translation in the given domain for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetND(dom, str, plural string, n int, vars ...interface{}) string {
	// Sync read
	l.RLock()
	defer l.RUnlock()

	if l.Domains != nil {
		if _, ok := l.Domains[dom]; ok {
			if l.Domains[dom] != nil {
				return l.Domains[dom].GetN(str, plural, n, vars...)
			}
		}
	}

	// Return the same we received by default
	return Printf(plural, vars...)
}

// GetC uses a domain "default" to return the corresponding Translation of the given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetC(str, ctx string, vars ...interface{}) string {
	return l.GetDC(l.GetDomain(), str, ctx, vars...)
}

// GetNC retrieves the (N)th plural form of Translation for the given string in the given context in the "default" domain.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetNC(str, plural string, n int, ctx string, vars ...interface{}) string {
	return l.GetNDC(l.GetDomain(), str, plural, n, ctx, vars...)
}

// GetDC returns the corresponding Translation in the given domain for the given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetDC(dom, str, ctx string, vars ...interface{}) string {
	return l.GetNDC(dom, str, str, 1, ctx, vars...)
}

// GetNDC retrieves the (N)th plural form of Translation in the given domain for the given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (l *Locale) GetNDC(dom, str, plural string, n int, ctx string, vars ...interface{}) string {
	// Sync read
	l.RLock()
	defer l.RUnlock()

	if l.Domains != nil {
		if _, ok := l.Domains[dom]; ok {
			if l.Domains[dom] != nil {
				return l.Domains[dom].GetNC(str, plural, n, ctx, vars...)
			}
		}
	}

	// Return the same we received by default
	return Printf(plural, vars...)
}
