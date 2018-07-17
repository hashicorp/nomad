package raw_exec

import (
	"testing"

	"github.com/hashicorp/nomad/plugins/drivers/raw-exec-plugin/proto"
	"github.com/stretchr/testify/require"
)

func TestRawExecDriver_StartOpen_Wait(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	execCtx := &proto.ExecContext{}
	taskInfo := &proto.TaskInfo{}

	d := NewRawExecDriver()
	resp, err := d.Start(execCtx, taskInfo)
	require.Nil(err)
	require.NotNil(resp)
}
