package text

import "github.com/leonelquinteros/gotext"

type ErrInputOverflow struct{}

func (e ErrInputOverflow) Error() string {
	return gotext.Get("input too long")
}
