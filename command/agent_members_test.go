package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestAgentMembersCommand_Implements(t *testing.T) {
	var _ cli.Command = &AgentMembersCommand{}
}
