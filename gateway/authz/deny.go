package authz

import (
	"errors"
	"fmt"
)

var ErrDenied = errors.New("gateway authz: access denied")

type DeniedError struct {
	Decision Decision
}

func (e *DeniedError) Error() string {
	if e == nil {
		return ErrDenied.Error()
	}
	return fmt.Sprintf("%s: operation=%s reason=%s", ErrDenied, e.Decision.Operation, e.Decision.Reason)
}

func (e *DeniedError) Unwrap() error { return ErrDenied }

func Denied(decision Decision) error { return &DeniedError{Decision: decision} }
func IsDenied(err error) bool        { return errors.Is(err, ErrDenied) }
