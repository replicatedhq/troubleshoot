package collect

import (
	"fmt"

	"github.com/pkg/errors"
)

type RBACError struct {
	DisplayName string
	Namespace   string
	Resource    string
	Verb        string
}

func (e RBACError) Error() string {
	if e.Namespace == "" {
		return fmt.Sprintf("cannot collect %s: action %q is not allowed on resource %q at the cluster scope", e.DisplayName, e.Verb, e.Resource)
	}
	return fmt.Sprintf("cannot collect %s: action %q is not allowed on resource %q in the %q namespace", e.DisplayName, e.Verb, e.Resource, e.Namespace)
}

func IsRBACError(err error) bool {
	_, ok := errors.Cause(err).(RBACError)
	return ok
}
