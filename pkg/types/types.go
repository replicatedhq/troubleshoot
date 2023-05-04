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

type SpecIssueError struct {
	Msg string
}

func (e *SpecIssueError) Error() string {
	return e.Msg
}

func (e *SpecIssueError) ExitStatus() int {
	return 2
}

func NewSpecIssueError(msg string) *SpecIssueError {
	return &SpecIssueError{Msg: msg}
}
