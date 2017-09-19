package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelListCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SentinelListCommand{}
}
