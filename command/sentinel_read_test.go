package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelReadCommand_Implements(t *testing.T) {
	var _ cli.Command = &SentinelReadCommand{}
}
