/*
 * Copyright (c) 2018 DeineAgentur UG https://www.deineagentur.com. All rights reserved.
 * Licensed under the MIT License. See LICENSE file in the project root for full license information.
 */

package gotext

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext/plurals"
)

const (
	MoMagicLittleEndian = 0x950412de // MoMagicLittleEndian encoding
	MoMagicBigEndian    = 0xde120495 // MoMagicBigEndian encoding

	EotSeparator = "\x04" // msgctxt and msgid separator
	NulSeparator = "\x00" // msgid and msgstr separator
)

/*
Mo parses the content of any MO file and provides all the Translation functions needed.
It's the base object used by all package methods.
And it's safe for concurrent use by multiple goroutines by using the sync package for locking.

Example:

	import (
		"fmt"
		"github.com/leonelquinteros/gotext"
	)

	func main() {
		// Create po object
		po := gotext.NewMoTranslator()

		// Parse .po file
		po.ParseFile("/path/to/po/file/translations.mo")

		// Get Translation
		fmt.Println(po.Get("Translate this"))
	}

*/
type Mo struct {
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

// NewMoTranslator creates a new Mo object with the Translator interface
func NewMoTranslator() Translator {
	return new(Mo)
}

// ParseFile tries to read the file by its provided path (f) and parse its content as a .po file.
func (mo *Mo) ParseFile(f string) {
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

	mo.Parse(data)
}

// Parse loads the translations specified in the provided string (str)
func (mo *Mo) Parse(buf []byte) {
	// Lock while parsing
	mo.Lock()

	// Init storage
	if mo.translations == nil {
		mo.translations = make(map[string]*Translation)
		mo.contexts = make(map[string]map[string]*Translation)
	}

	r := bytes.NewReader(buf)

	var magicNumber uint32
	if err := binary.Read(r, binary.LittleEndian, &magicNumber); err != nil {
		return
		// return fmt.Errorf("gettext: %v", err)
	}
	var bo binary.ByteOrder
	switch magicNumber {
	case MoMagicLittleEndian:
		bo = binary.LittleEndian
	case MoMagicBigEndian:
		bo = binary.BigEndian
	default:
		return
		// return fmt.Errorf("gettext: %v", "invalid magic number")
	}

	var header struct {
		MajorVersion uint16
		MinorVersion uint16
		MsgIDCount   uint32
		MsgIDOffset  uint32
		MsgStrOffset uint32
		HashSize     uint32
		HashOffset   uint32
	}
	if err := binary.Read(r, bo, &header); err != nil {
		return
		// return fmt.Errorf("gettext: %v", err)
	}
	if v := header.MajorVersion; v != 0 && v != 1 {
		return
		// return fmt.Errorf("gettext: %v", "invalid version number")
	}
	if v := header.MinorVersion; v != 0 && v != 1 {
		return
		// return fmt.Errorf("gettext: %v", "invalid version number")
	}

	msgIDStart := make([]uint32, header.MsgIDCount)
	msgIDLen := make([]uint32, header.MsgIDCount)
	if _, err := r.Seek(int64(header.MsgIDOffset), 0); err != nil {
		return
		// return fmt.Errorf("gettext: %v", err)
	}
	for i := 0; i < int(header.MsgIDCount); i++ {
		if err := binary.Read(r, bo, &msgIDLen[i]); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
		if err := binary.Read(r, bo, &msgIDStart[i]); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
	}

	msgStrStart := make([]int32, header.MsgIDCount)
	msgStrLen := make([]int32, header.MsgIDCount)
	if _, err := r.Seek(int64(header.MsgStrOffset), 0); err != nil {
		return
		// return fmt.Errorf("gettext: %v", err)
	}
	for i := 0; i < int(header.MsgIDCount); i++ {
		if err := binary.Read(r, bo, &msgStrLen[i]); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
		if err := binary.Read(r, bo, &msgStrStart[i]); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
	}

	for i := 0; i < int(header.MsgIDCount); i++ {
		if _, err := r.Seek(int64(msgIDStart[i]), 0); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
		msgIdData := make([]byte, msgIDLen[i])
		if _, err := r.Read(msgIdData); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}

		if _, err := r.Seek(int64(msgStrStart[i]), 0); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}
		msgStrData := make([]byte, msgStrLen[i])
		if _, err := r.Read(msgStrData); err != nil {
			return
			// return fmt.Errorf("gettext: %v", err)
		}

		if len(msgIdData) == 0 {
			mo.addTranslation(msgIdData, msgStrData)
		} else {
			mo.addTranslation(msgIdData, msgStrData)
		}
	}

	// Unlock to parse headers
	mo.Unlock()

	// Parse headers
	mo.parseHeaders()
	return
	// return nil
}

func (mo *Mo) addTranslation(msgid, msgstr []byte) {
	translation := NewTranslation()
	var msgctxt []byte
	var msgidPlural []byte

	d := bytes.Split(msgid, []byte(EotSeparator))
	if len(d) == 1 {
		msgid = d[0]
	} else {
		msgid, msgctxt = d[1], d[0]
	}

	dd := bytes.Split(msgid, []byte(NulSeparator))
	if len(dd) > 1 {
		msgid = dd[0]
		dd = dd[1:]
	}

	translation.ID = string(msgid)

	msgidPlural = bytes.Join(dd, []byte(NulSeparator))
	if len(msgidPlural) > 0 {
		translation.PluralID = string(msgidPlural)
	}

	ddd := bytes.Split(msgstr, []byte(NulSeparator))
	if len(ddd) > 0 {
		for i, s := range ddd {
			translation.Trs[i] = string(s)
		}
	}

	if len(msgctxt) > 0 {
		// With context...
		if _, ok := mo.contexts[string(msgctxt)]; !ok {
			mo.contexts[string(msgctxt)] = make(map[string]*Translation)
		}
		mo.contexts[string(msgctxt)][translation.ID] = translation
	} else {
		mo.translations[translation.ID] = translation
	}
}

// parseHeaders retrieves data from previously parsed headers
func (mo *Mo) parseHeaders() {
	// Make sure we end with 2 carriage returns.
	raw := mo.Get("") + "\n\n"

	// Read
	reader := bufio.NewReader(strings.NewReader(raw))
	tp := textproto.NewReader(reader)

	var err error

	// Sync Headers write.
	mo.Lock()
	defer mo.Unlock()

	mo.Headers, err = tp.ReadMIMEHeader()
	if err != nil {
		return
	}

	// Get/save needed headers
	mo.Language = mo.Headers.Get("Language")
	mo.PluralForms = mo.Headers.Get("Plural-Forms")

	// Parse Plural-Forms formula
	if mo.PluralForms == "" {
		return
	}

	// Split plural form header value
	pfs := strings.Split(mo.PluralForms, ";")

	// Parse values
	for _, i := range pfs {
		vs := strings.SplitN(i, "=", 2)
		if len(vs) != 2 {
			continue
		}

		switch strings.TrimSpace(vs[0]) {
		case "nplurals":
			mo.nplurals, _ = strconv.Atoi(vs[1])

		case "plural":
			mo.plural = vs[1]

			if expr, err := plurals.Compile(mo.plural); err == nil {
				mo.pluralforms = expr
			}

		}
	}
}

// pluralForm calculates the plural form index corresponding to n.
// Returns 0 on error
func (mo *Mo) pluralForm(n int) int {
	mo.RLock()
	defer mo.RUnlock()

	// Failure fallback
	if mo.pluralforms == nil {
		/* Use the Germanic plural rule.  */
		if n == 1 {
			return 0
		}
		return 1

	}
	return mo.pluralforms.Eval(uint32(n))
}

// Get retrieves the corresponding Translation for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (mo *Mo) Get(str string, vars ...interface{}) string {
	// Sync read
	mo.RLock()
	defer mo.RUnlock()

	if mo.translations != nil {
		if _, ok := mo.translations[str]; ok {
			return Printf(mo.translations[str].Get(), vars...)
		}
	}

	// Return the same we received by default
	return Printf(str, vars...)
}

// GetN retrieves the (N)th plural form of Translation for the given string.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (mo *Mo) GetN(str, plural string, n int, vars ...interface{}) string {
	// Sync read
	mo.RLock()
	defer mo.RUnlock()

	if mo.translations != nil {
		if _, ok := mo.translations[str]; ok {
			return Printf(mo.translations[str].GetN(mo.pluralForm(n)), vars...)
		}
	}

	if n == 1 {
		return Printf(str, vars...)
	}
	return Printf(plural, vars...)
}

// GetC retrieves the corresponding Translation for a given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (mo *Mo) GetC(str, ctx string, vars ...interface{}) string {
	// Sync read
	mo.RLock()
	defer mo.RUnlock()

	if mo.contexts != nil {
		if _, ok := mo.contexts[ctx]; ok {
			if mo.contexts[ctx] != nil {
				if _, ok := mo.contexts[ctx][str]; ok {
					return Printf(mo.contexts[ctx][str].Get(), vars...)
				}
			}
		}
	}

	// Return the string we received by default
	return Printf(str, vars...)
}

// GetNC retrieves the (N)th plural form of Translation for the given string in the given context.
// Supports optional parameters (vars... interface{}) to be inserted on the formatted string using the fmt.Printf syntax.
func (mo *Mo) GetNC(str, plural string, n int, ctx string, vars ...interface{}) string {
	// Sync read
	mo.RLock()
	defer mo.RUnlock()

	if mo.contexts != nil {
		if _, ok := mo.contexts[ctx]; ok {
			if mo.contexts[ctx] != nil {
				if _, ok := mo.contexts[ctx][str]; ok {
					return Printf(mo.contexts[ctx][str].GetN(mo.pluralForm(n)), vars...)
				}
			}
		}
	}

	if n == 1 {
		return Printf(str, vars...)
	}
	return Printf(plural, vars...)
}
