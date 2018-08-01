package raw_exec

import "github.com/hashicorp/nomad/plugins/drivers/raw-exec/proto"

func NewRawExecDriver() *RawExec {
	return &RawExec{}
}

type RawExec struct{}

func (RawExec) Start(ctx *proto.ExecContext, task *proto.TaskInfo) (*proto.StartResponse, error) {
	res := &proto.StartResponse{TaskId: "12345"}
	return res, nil
}
