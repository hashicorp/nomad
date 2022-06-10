package command

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
)

func TestVersionCommand_implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &VersionCommand{}
}
