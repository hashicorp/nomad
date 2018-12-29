package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestServerForceLeaveCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ServerForceLeaveCommand{}
}
