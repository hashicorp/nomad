package command

import (
	"testing"

	"github.com/hashicorp/nomad/Godeps/_workspace/src/github.com/mitchellh/cli"
)

func TestVersionCommand_implements(t *testing.T) {
	var _ cli.Command = &VersionCommand{}
}
