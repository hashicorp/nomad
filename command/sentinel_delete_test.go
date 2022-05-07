package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestSentinelDeleteCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &SentinelDeleteCommand{}
}
