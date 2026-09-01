package verifier

import "errors"

type remoteFallbackError struct {
	err error
}

func (e *remoteFallbackError) Error() string { return e.err.Error() }
func (e *remoteFallbackError) Unwrap() error { return e.err }

func allowRemoteFallback(err error) error {
	if err == nil {
		return nil
	}
	return &remoteFallbackError{err: err}
}

func canFallbackRemotely(err error) bool {
	var target *remoteFallbackError
	return errors.As(err, &target)
}
