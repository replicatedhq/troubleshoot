package logger

import (
	"fmt"
)

var (
	quiet = false
)

func SetQuiet(s bool) {
	quiet = s
}

func Printf(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf(format, args...)
}
