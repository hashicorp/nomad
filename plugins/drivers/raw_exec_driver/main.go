package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"
	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/shared"
)

func main() {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: shared.Handshake,
		Plugins:         shared.PluginMap,
		Cmd:             exec.Command("sh", "-c", "./raw-exec-go-grpc"),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC},
	})
	defer client.Kill()

	rpcClient, err := client.Client()
	if err != nil {
		fmt.Println("Error when trying to start rpc client:", err.Error())
		os.Exit(1)
	}

	raw, err := rpcClient.Dispense("raw_exec")
	if err != nil {
		fmt.Println("Error when dispensing raw_exec:", err.Error())
		os.Exit(1)
	} else if raw == nil {
		fmt.Println("Error when dispensing raw_exec: got null instead of interface")
		os.Exit(1)
	}

	rawExec := raw.(shared.RawExec)
	execCtx := &proto.ExecContext{}
	taskInfo := &proto.TaskInfo{}

	result, err := rawExec.NewStart(execCtx, taskInfo)
	if err != nil {
		fmt.Printf("Encountered errors: %s \n", err.Error())
	}

	fmt.Printf(" Found task id: %s \n", result.TaskId)
}
