package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestSentinelApplyCommand_Implements(t *testing.T) {
	var _ cli.Command = &SentinelApplyCommand{}
}
