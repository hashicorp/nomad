package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

var _ cli.Command = &LicensePutCommand{}

func TestCommand_LicensePut(t *testing.T) {
	// TODO create test once http endpoints are configured
}
