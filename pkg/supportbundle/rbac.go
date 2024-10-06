package supportbundle

import "fmt"

// Custom error type for RBAC permission errors
type RBACPermissionError struct {
	Forbidden []error
}

func (e *RBACPermissionError) Error() string {
	return fmt.Sprintf("insufficient permissions: %v", e.Forbidden)
}

func (e *RBACPermissionError) HasErrors() bool {
	return len(e.Forbidden) > 0
}
