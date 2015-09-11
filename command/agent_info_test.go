package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestAgentInfoCommand_Implements(t *testing.T) {
	var _ cli.Command = &AgentInfoCommand{}
}

func TestAgentInfoCommand_Run(t *testing.T) {
	agent := testAgent(t)
	defer agent.Shutdown()
	println("yay")
}
