package types

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return e.Name + ": not found"
}
