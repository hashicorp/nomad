package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelApplyCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &SentinelApplyCommand{}
}
