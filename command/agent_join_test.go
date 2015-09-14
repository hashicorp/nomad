package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestAgentJoinCommand_Implements(t *testing.T) {
	var _ cli.Command = &AgentJoinCommand{}
}
