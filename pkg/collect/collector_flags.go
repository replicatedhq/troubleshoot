package collect

type CollectorFlags uint64

// CollectorMode values
// These values are used to define the modes a collector can run in.
// e.g if a collector requires root access, the RequireRoot mode can be set.
// Multiple flags can be set for a collector. The values are bit flags,
// so they can be combined easily. There is a maximum of 64 modes which can be defined.
// Example of creating combined modes
//
//	m := RequireRoot | AnotherMode
//
// Example of checking if a mode is set
//
//	if m&RequireRoot != 0 {}
//
// Example of appending modes
//
//	m =| RequireRoot
//	m = m | AnotherMode
const (
	RequireRoot CollectorFlags = 1 << (64 - 1 - iota)
)

var EmptyFlags CollectorFlags = 0

func (cm CollectorFlags) RequiresRoot() bool {
	return cm&RequireRoot != 0
}
