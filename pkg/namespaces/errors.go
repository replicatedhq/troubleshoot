package namespaces

import "fmt"

// WrapIfFail executes the provided function. If the function succeeds it
// simply returns the original error (that can be nil). If the function fails
// then it assesses if an original error was provided and wraps it if true.
// This function is a sugar to be used at when deferring function that can also
// return errors, we don't want to loose any context.
func WrapIfFail(msg string, originalerr error, fn func() error) error {
	if fnerr := fn(); fnerr != nil {
		if originalerr == nil {
			return fmt.Errorf("%s: %w", msg, fnerr)
		}
		return fmt.Errorf("%s: %w: %w", msg, fnerr, originalerr)
	}
	return originalerr
}
