package driver

import (
	ctestutils "github.com/hashicorp/nomad/client/testutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/nomad/structs"
)

type ExecDriverIT struct {
	framework.TC
}

func (tc *ExecDriverIT) BeforeAll(f *framework.F) {
	ctestutils.ExecCompatible(f.T())
}
func (tc *ExecDriverIT) BeforeEach(f *framework.F) {
	task := &structs.Task{
		Name:      "foo",
		Driver:    "exec",
		Resources: structs.DefaultResources(),
	}
	ctx := testDriverContexts(t, task)
	defer ctx.AllocDir.Destroy()
	d := NewExecDriver(ctx.DriverCtx)
	node := &structs.Node{
		Attributes: map[string]string{
			"unique.cgroup.mountpoint": "/sys/fs/cgroup",
		},
	}

}
