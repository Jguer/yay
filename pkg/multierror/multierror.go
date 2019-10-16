package multierror

import "sync"

// MultiError type handles error accumulation from goroutines
type MultiError struct {
	Errors []error
	mux    sync.Mutex
}

// Error turns the MultiError structure into a string
func (err *MultiError) Error() string {
	str := ""

	for _, e := range err.Errors {
		str += e.Error() + "\n"
	}

	return str[:len(str)-1]
}

// Add adds an error to the Multierror structure
func (err *MultiError) Add(e error) {
	if e == nil {
		return
	}

	err.mux.Lock()
	err.Errors = append(err.Errors, e)
	err.mux.Unlock()
}

// Return is used as a wrapper on return on whether to return the
// MultiError Structure if errors exist or nil instead of delivering an empty structure
func (err *MultiError) Return() error {
	if len(err.Errors) > 0 {
		return err
	}

	return nil
}
