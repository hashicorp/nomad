package driver

import (
	"log"
	"os"

	"github.com/hashicorp/nomad/client/config"
)

func testLogger() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}

func testConfig() *config.Config {
	return &config.Config{}
}
