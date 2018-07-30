package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-plugin"

	"strings"

	"github.com/golang/protobuf/jsonpb"
	_struct "github.com/golang/protobuf/ptypes/struct"
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

	currentDir, err := os.Getwd() // TODO
	if err != nil {
		panic(fmt.Sprintf("encoungered error when getting current dir: %s", err.Error()))
	}

	execCtx := &proto.ExecContext{
		TaskDir: &proto.TaskDir{
			Directory: currentDir,
			LogDir:    currentDir,
			LogLevel:  "DEBUG",
		},
		TaskEnv: &proto.TaskEnv{},
	}
	jsonConfig := `{
                    "Command":"echo",
                    "Args":["the", "quick", "brown", "fox", "jumped"]
                   }`
	unMarshaller := jsonpb.Unmarshaler{AllowUnknownFields: false}

	reader := strings.NewReader(jsonConfig)
	structConfig := &_struct.Struct{}
	err = unMarshaller.Unmarshal(reader, structConfig)

	if err != nil {
		fmt.Println("Error unmarshalling json into protobuf Struct:%v", err)
		os.Exit(-1)
	}
	taskInfo := &proto.TaskInfo{
		Resources: &proto.Resources{
			Cpu:      250,
			MemoryMb: 256,
			DiskMb:   20,
		},
		LogConfig: &proto.LogConfig{
			MaxFiles:      10,
			MaxFileSizeMb: 10,
		},
		Config: structConfig,
	}

	result, err := rawExec.NewStart(execCtx, taskInfo)
	if err != nil {
		fmt.Printf("Encountered errors: %s \n", err.Error())
	}

	fmt.Printf(": %s \n", result)
}
