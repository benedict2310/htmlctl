package cli

import "errors"

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func exitCodeError(code int, err error) error {
	if code <= 0 {
		return err
	}
	return &ExitError{Code: code, Err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded *ExitError
	if errors.As(err, &coded) && coded.Code > 0 {
		return coded.Code
	}
	return 1
}
