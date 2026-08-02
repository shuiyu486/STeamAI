package workstream

import "errors"

type ZeroProgressError struct {
	cause error
}

func (e ZeroProgressError) Error() string {
	return e.cause.Error()
}

func (e ZeroProgressError) Unwrap() error {
	return e.cause
}

func MarkZeroProgress(err error) error {
	if err == nil || IsZeroProgress(err) {
		return err
	}
	return ZeroProgressError{cause: err}
}

func IsZeroProgress(err error) bool {
	var zeroProgress ZeroProgressError
	return errors.As(err, &zeroProgress)
}
