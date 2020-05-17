package command

import (
	"testing"

	"github.com/mitchellh/cli"
)

func TestOperator_Raft_Implements(t *testing.T) {
	var _ cli.Command = &OperatorRaftCommand{}
}
