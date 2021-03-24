package swag

import (
	"log"
)

const (
	test = iota
	release
)

var swagMode = release

func isRelease() bool {
	return swagMode == release
}

// Println calls Output to print to the standard logger when release mode.
func Println(v ...interface{}) {
	if isRelease() {
		log.Println(v...)
	}
}

// Printf calls Output to print to the standard logger when release mode.
func Printf(format string, v ...interface{}) {
	if isRelease() {
		log.Printf(format, v...)
	}
}
