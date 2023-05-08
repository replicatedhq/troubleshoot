package types

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return e.Name + ": not found"
}

type ExitError interface {
	Error() string
	ExitStatus() int
}

type ExitCodeError struct {
	Msg  string
	Code int
}

func (e *ExitCodeError) Error() string {
	return e.Msg
}

func (e *ExitCodeError) ExitStatus() int {
	return e.Code
}

func NewExitCodeError(exitCode int, theErr error) *ExitCodeError {
	useErr := ""
	if theErr != nil {
		useErr = theErr.Error()
	}
	return &ExitCodeError{Msg: useErr, Code: exitCode}
}
