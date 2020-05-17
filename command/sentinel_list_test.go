package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelListCommand_Implements(t *testing.T) {
	var _ cli.Command = &SentinelListCommand{}
}
