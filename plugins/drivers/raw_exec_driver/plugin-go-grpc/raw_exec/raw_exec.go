package raw_exec

import "github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"

// TODO do we need to inject a driver context? Probably not as drivers are now not 1:1 with tasks
func NewRawExecDriver() *RawExec {
	return &RawExec{}
}

type RawExec struct{}

func (RawExec) Start(ctx *proto.ExecContext, task *proto.TaskInfo) (*proto.StartResponse, error) {
	res := &proto.StartResponse{TaskId: "12345"}
	return res, nil
}
