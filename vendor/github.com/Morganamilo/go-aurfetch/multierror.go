package aurfetch

type MultiError struct {
	Errors []error
}

func (err *MultiError) Error() string {
	str := ""

	for _, e := range err.Errors {
		str += e.Error() + "\n"
	}

	return str[:len(str)-1]
}

func (err *MultiError) Add(e error) {
	err.Errors = append(err.Errors, e)
}

func (err *MultiError) Return() *MultiError {
	if len(err.Errors) > 0 {
		return err
	}

	return nil
}
