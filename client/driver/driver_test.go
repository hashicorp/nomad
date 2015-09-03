package driver

import (
	"log"
	"os"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}
