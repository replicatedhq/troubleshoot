package analyzer

import "fmt"

type ComparisonOperator int

const (
	Unknown ComparisonOperator = iota
	Equal
	NotEqual
	GreaterThan
	GreaterThanOrEqual
	LessThan
	LessThanOrEqual
)

func ParseComparisonOperator(s string) (ComparisonOperator, error) {
	switch s {
	case "=", "==", "===":
		return Equal, nil
	case "!=", "!==":
		return NotEqual, nil
	case "<":
		return LessThan, nil
	case ">":
		return GreaterThan, nil
	case "<=":
		return LessThanOrEqual, nil
	case ">=":
		return GreaterThanOrEqual, nil
	}

	return Unknown, fmt.Errorf("unknown operator: %s", s)
}
