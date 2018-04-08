/*
 * Copyright (c) 2018 DeineAgentur UG https://www.deineagentur.com. All rights reserved.
 * Licensed under the MIT License. See LICENSE file in the project root for full license information.
 */

package gotext

import (
	"bufio"
	"io/ioutil"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext/plurals"
)

/*
Po parses the content of any PO file and provides all the Translation functions needed.
It's the base object used by all package methods.
And it's safe for concurrent use by multiple goroutines by using the sync package for locking.

Example:

	import (
		"fmt"
		"github.com/leonelquinteros/gotext"
	)

	func main() {
		// Create po object
		po := gotext.NewPoTranslator()

		// Parse .po file
		po.ParseFile("/path/to/po/file/translations.po")

		// Get Translation
		fmt.Println(po.Get("Translate this"))
	}

*/
type Po struct {
	// Headers storage
	Headers textproto.MIMEHeader

	// Language header
	Language string

	// Plural-Forms header
	PluralForms string

	// Parsed Plural-Forms header values
	nplurals    int
	plural      string
	pluralforms plurals.Expression

	// Storage
	translations map[string]*Translation
	contexts     map[string]map[string]*Translation

	// Sync Mutex
	sync.RWMutex

	// Parsing buffers
	trBuffer  *Translation
	ctxBuffer string
}

type parseState int

const (
	head parseState = iota
	msgCtxt
	msgID
	msgIDPlural
	msgStr
)

// NewPoTranslator creates a new Po object with the Translator interface
func NewPoTranslator() Translator {
	return new(Po)
}

// ParseFile tries to read the file by its provided path (f) and parse its content as a .po file.
func (po *Po) ParseFile(f string) {
	// Check if file exists
	info, err := os.Stat(f)
	if err != nil {
		return
	}

	// Check that isn't a directory
	if info.IsDir() {
		return
	}

	// Parse file content
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return
	}

	po.Parse(data)
}

// Parse loads the translations specified in the provided string (str)
func (po *Po) Parse(buf []byte) {
	// Lock while parsing
	po.Lock()

	// Init storage
	if po.translations == nil {
		po.translations = make(map[string]*Translation)
		po.contexts = make(map[string]map[string]*Translation)
	}

	// Get lines
	lines := strings.Split(string(buf), "\n")

	// Init buffer
	po.trBuffer = NewTranslation()
	po.ctxBuffer = ""

	state := head
	for _, l := range lines {
		// Trim spaces
		l = strings.TrimSpace(l)

		// Skip invalid lines
		if !po.isValidLine(l) {
			continue
		}

		// Buffer context and continue
		if strings.HasPrefix(l, "msgctxt") {
			po.parseContext(l)
			state = msgCtxt
			continue
		}

		// Buffer msgid and continue
		if strings.HasPrefix(l, "msgid") && !strings.HasPrefix(l, "msgid_plural") {
			po.parseID(l)
			state = msgID
			continue
		}

		// Check for plural form
		if strings.HasPrefix(l, "msgid_plural") {
			po.parsePluralID(l)
			state = msgIDPlural
			continue
		}

		// Save Translation
		if strings.HasPrefix(l, "msgstr") {
			po.parseMessage(l)
			state = msgStr
			continue
		}

		// Multi line strings and headers
		if strings.HasPrefix(l, "\"") && strings.HasSuffix(l, "\"") {
			po.parseString(l, state)
			continue
		}
	}

	// Save last Translation buffer.
	po.saveBuffer()

	// Unlock to parse headers
	po.Unlock()

	// Parse headers
	po.parseHeaders()
}

// saveBuffer takes the context and Translation buffers
// and saves it on the translations collection
func (po *Po) saveBuffer() {
	// With no context...
	if po.ctxBuffer == "" {
		po.translations[po.trBuffer.ID] = po.trBuffer
	} else {
		// With context...
		if _, ok := po.contexts[po.ctxBuffer]; !ok {
			po.contexts[po.ctxBuffer] = make(map[string]*Translation)
		}
		po.contexts[po.ctxBuffer][po.trBuffer.ID] = po.trBuffer

		// Cleanup current context buffer if needed
		if po.trBuffer.ID != "" {
			po.ctxBuffer = ""
		}
	}

	// Flush Translation buffer
	po.trBuffer = NewTranslation()
}

// parseContext takes a line starting with "msgctxt",
// saves the current Translation buffer and creates a new context.
func (po *Po) parseContext(l string) {
	// Save current Translation buffer.
	po.saveBuffer()

	// Buffer context
	po.ctxBuffer, _ = strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(l, "msgctxt")))
}

// parseID takes a line starting with "msgid",
// saves the current Translation and creates a new msgid buffer.
func (po *Po) parseID(l string) {
	// Save current Translation buffer.
	po.saveBuffer()

	// Set id
	po.trBuffer.ID, _ = strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(l, "msgid")))
}

// parsePluralID saves the plural id buffer from a line starting with "msgid_plural"
func (po *Po) parsePluralID(l string) {
	po.trBuffer.PluralID, _ = strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(l, "msgid_plural")))
}

// parseMessage takes a line starting with "msgstr" and saves it into the current buffer.
func (po *Po) parseMessage(l string) {
	l = strings.TrimSpace(strings.TrimPrefix(l, "msgstr"))

	// Check for indexed Translation forms
	if strings.HasPrefix(l, "[") {
		idx := strings.Index(l, "]")
		if idx == -1 {
			// Skip wrong index formatting
			return
		}

		// Parse index
		i, err := strconv.Atoi(l[1:idx])
		if err != nil {
			// Skip wrong index formatting
			return
		}

		// Parse Translation string
		po.trBuffer.Trs[i], _ = strconv.Unquote(strings.TrimSpace(l[idx+1:]))

		// Loop
		return
	}

	// Save single Translation form under 0 index
	po.trBuffer.Trs[0], _ = strconv.Unquote(l)
}

// parseString takes a well formatted string without prefix
// and creates headers or attach multi-line strings when corresponding
func (po *Po) parseString(l string, state parseState) {
	clean, _ := strconv.Unquote(l)

	switch state {
	case msgStr:
		// Append to last Translation found
		po.trBuffer.Trs[len(po.trBuffer.Trs)-1] += clean

	case msgID:
		// Multiline msgid - Append to current id
		po.trBuffer.ID += clean

	case msgIDPlural:
		// Multiline msgid - Append to current id
		po.trBuffer.PluralID += clean

	case msgCtxt:
		// Multiline context - Append to current context
		po.ctxBuffer += clean

	}
}

// isValidLine checks for line prefixes to detect valid syntax.
func (po *Po) isValidLine(l string) bool {
	// Check prefix
	valid := []string{
		"\"",
		"msgctxt",
		"msgid",
		"msgid_plural",
		"msgstr",
	}

	for _, v := range valid {
		if strings.HasPrefix(l, v) {
			return true
		}
	}

	return false
}

// parseHeaders retrieves data from previously parsed headers
func (po *Po) parseHeaders() {
	// Make sure we end with 2 carriage returns.
	raw := po.Get("") + "\n\n"

	// Read
	reader := bufio.NewReader(strings.NewReader(raw))
	tp := textproto.NewReader(reader)

	var err error

	// Sync Headers write.
	po.Lock()
	defer po.Unlock()

	po.Headers, err = tp.ReadMIMEHeader()
	if err != nil {
		return
	}

	// Get/save needed headers
	po.Language = po.Headers.Get("Language")
	po.PluralForms = po.Headers.Get("Plural-Forms")

	// Parse Plural-Forms formula
	if po.PluralForms == "" {
		return
	}

	// Split plural form header value
	pfs := strings.Split(po.PluralForms, ";")

	// Parse values
	for _, i := range pfs {
		vs := strings.SplitN(i, "=", 2)
		if len(vs) != 2 {
			continue
		}

		switch strings.TrimSpace(vs[0]) {
		case "nplurals":
			po.nplurals, _ = strconv.Atoi(vs[1])

		case "plural":
			po.plural = vs[1]

			if expr, err := plurals.Compile(po.plural); err == nil {
				po.pluralforms = expr
			}

		}
	}
}

// pluralForm calculates the plural form index corresponding to n.
// Returns 0 on error
func (po *Po) pluralForm(n int) int {
	po.RLock()
	defer po.RUnlock()

	// Failure fallback
	if po.pluralforms == nil {
		/* Use the Germanic plural rule.  */
		if n == 1 {
			return 0
		} else {
			return 1
		}
	}
	return po.pluralforms.Eval(uint32(n))
}

// Get retrieves the corresponding Translation for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (po *Po) Get(str string, vars ...interface{}) string {
	// Sync read
	po.RLock()
	defer po.RUnlock()

	if po.translations != nil {
		if _, ok := po.translations[str]; ok {
			return Printf(po.translations[str].Get(), vars...)
		}
	}

	// Return the same we received by default
	return Printf(str, vars...)
}

// GetN retrieves the (N)th plural form of Translation for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (po *Po) GetN(str, plural string, n int, vars ...interface{}) string {
	// Sync read
	po.RLock()
	defer po.RUnlock()

	if po.translations != nil {
		if _, ok := po.translations[str]; ok {
			return Printf(po.translations[str].GetN(po.pluralForm(n)), vars...)
		}
	}

	if n == 1 {
		return Printf(str, vars...)
	}
	return Printf(plural, vars...)
}

// GetC retrieves the corresponding Translation for a given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (po *Po) GetC(str, ctx string, vars ...interface{}) string {
	// Sync read
	po.RLock()
	defer po.RUnlock()

	if po.contexts != nil {
		if _, ok := po.contexts[ctx]; ok {
			if po.contexts[ctx] != nil {
				if _, ok := po.contexts[ctx][str]; ok {
					return Printf(po.contexts[ctx][str].Get(), vars...)
				}
			}
		}
	}

	// Return the string we received by default
	return Printf(str, vars...)
}

// GetNC retrieves the (N)th plural form of Translation for the given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (po *Po) GetNC(str, plural string, n int, ctx string, vars ...interface{}) string {
	// Sync read
	po.RLock()
	defer po.RUnlock()

	if po.contexts != nil {
		if _, ok := po.contexts[ctx]; ok {
			if po.contexts[ctx] != nil {
				if _, ok := po.contexts[ctx][str]; ok {
					return Printf(po.contexts[ctx][str].GetN(po.pluralForm(n)), vars...)
				}
			}
		}
	}

	if n == 1 {
		return Printf(str, vars...)
	}
	return Printf(plural, vars...)
}
