package errorExt

import "github.com/pkg/errors"

type ErrorHandler struct {
	Name string
	Handler func() error
}

func HandleErr(errs []ErrorHandler) error {
	for _, obj := range errs {
		if err := obj.Handler(); err != nil {
			return errors.Wrap(err, obj.Name)
		}
	}
	return nil
}
