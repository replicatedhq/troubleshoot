package convert

import (
	"github.com/pkg/errors"
)

type FuncError struct {
	Name string // Name of function.
	Err  error  // Pre-formatted error.
}

func (e FuncError) Error() string {
	return e.Err.Error()
}

// Panic will panic with a recoverable error.
func Panic(name string, err error) error {
	panic(Error(name, err))
}

// Error will wrap a template error in an ExecError causing template.Execute to recover.
func Error(name string, err error) error {
	return FuncError{
		Name: name,
		Err:  err,
	}
}

func IsFuncError(err error) bool {
	_, ok := errors.Cause(err).(FuncError)
	return ok
}
