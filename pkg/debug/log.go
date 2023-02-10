package debug

import (
	"log"

	"github.com/spf13/viper"
)

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
