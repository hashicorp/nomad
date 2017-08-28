package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelWriteCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SentinelWriteCommand{}
}
