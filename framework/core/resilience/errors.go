package resilience

import (
	"errors"
	"fmt"
)

var (
	ErrTimeoutBudgetExceeded = errors.New("resilience: timeout budget exceeded")
	ErrCircuitOpen           = errors.New("resilience: circuit open")
	ErrRateLimited           = errors.New("resilience: rate limited")
	ErrLoadShed              = errors.New("resilience: load shed")
)

// Rejection identifies a policy rejection without losing errors.Is semantics.
type Rejection struct {
	Policy string
	Key    string
	Err    error
}

func (err *Rejection) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.Key == "" {
		return fmt.Sprintf("%s: %v", err.Policy, err.Err)
	}
	return fmt.Sprintf("%s[%s]: %v", err.Policy, err.Key, err.Err)
}

func (err *Rejection) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func reject(policy, key string, err error) error {
	return &Rejection{Policy: policy, Key: key, Err: err}
}
