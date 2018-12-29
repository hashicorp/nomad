package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestACLBootstrapCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ACLBootstrapCommand{}
}
