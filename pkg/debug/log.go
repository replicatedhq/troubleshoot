package debug

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

var logger = log.New(os.Stderr, "[debug]", log.Lshortfile)

func Print(v ...interface{}) {
	if viper.GetBool("debug") {
		log.Print(v...)
	}
}

func Printf(format string, v ...interface{}) {
	if viper.GetBool("debug") {
		log.Printf(format, v...)
	}
}

func Println(v ...interface{}) {
	if viper.GetBool("debug") {
		log.Println(v...)
	}
}
