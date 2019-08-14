package types

import "sync"

// MultiError type handles error accumulation from goroutines
type MultiError struct {
	Errors []error
	mux    sync.Mutex
}

func (err *MultiError) Error() string {
	str := ""

	for _, e := range err.Errors {
		str += e.Error() + "\n"
	}

	return str[:len(str)-1]
}

func (err *MultiError) Add(e error) {
	if e == nil {
		return
	}

	err.mux.Lock()
	err.Errors = append(err.Errors, e)
	err.mux.Unlock()
}

func (err *MultiError) Return() error {
	if len(err.Errors) > 0 {
		return err
	}

	return nil
}
