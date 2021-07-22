package logger

import (
	"log"
	"os"
)

var (
	logger *log.Logger
	quiet  = false
)

func init() {
	logger = log.New(os.Stderr, "", log.LstdFlags)
}

func SetQuiet(s bool) {
	quiet = s
}

func Printf(format string, args ...interface{}) {
	if quiet {
		return
	}
	logger.Printf(format, args...)
}
