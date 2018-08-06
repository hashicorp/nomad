package raw_exec

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/proto"
)

func unmarshallExecContext(ctx *proto.ExecContext) *ExecContext {
	return &ExecContext{
		TaskEnv: &TaskEnv{},
		TaskDir: &TaskDir{
			Dir:       ctx.TaskDir.Directory,
			LogDir:    ctx.TaskDir.LogDir,
			LogLevel:  ctx.TaskDir.LogLevel,
			LogOutput: os.Stdout,
		},
		MaxPort:        5000,
		MinPort:        2000,
		MaxKillTimeout: time.Duration(40),
		Version:        "1.0", // TODO was d.DriverContext.Config.Version.VersionNumber()
	}
}

func unmarshallTaskInfo(tInfo *proto.TaskInfo) (*TaskInfo, error) {
	marshaller := jsonpb.Marshaler{EnumsAsInts: true, EmitDefaults: true, OrigName: false}

	configString, err := marshaller.MarshalToString(tInfo.Config)
	if err != nil {
		// TODO  should log to a logger here
		fmt.Println("Error decoding json config struct", err)
		return nil, err
	}
	rawExecTaskConfig := &RawExecTaskConfig{}
	if err := json.Unmarshal([]byte(configString), rawExecTaskConfig); err != nil {
		return nil, fmt.Errorf("Failed to parse config json '%s': %v", configString, err)
	}

	taskInfo := &TaskInfo{
		Resources: &Resources{
			CPU:      int(tInfo.Resources.Cpu),
			MemoryMB: int(tInfo.Resources.MemoryMb),
			DiskMB:   int(tInfo.Resources.DiskMb),
		},
		LogConfig: &LogConfig{
			MaxFiles:      int(tInfo.LogConfig.MaxFiles),
			MaxFileSizeMB: int(tInfo.LogConfig.MaxFileSizeMb),
		},
		Name: "taskName",
		Config: &Config{
			Command: rawExecTaskConfig.Command,
			Args:    rawExecTaskConfig.Args,
		},
	}
	command := taskInfo.Config.Command
	if err := validateCommand(command, "args"); err != nil {
		return nil, err
	}

	return taskInfo, nil
}

// unMarshallPluginReattachConfig returns a go-plugin reattach config from its protbuf representation
func unMarshallPluginReattachConfig(c *proto.PluginReattachInfo) *plugin.ReattachConfig {
	var addr net.Addr
	switch c.AddressNetwork {
	case "unix", "unixgram", "unixpacket":
		addr, _ = net.ResolveUnixAddr(c.AddressNetwork, c.AddressName)
	case "tcp", "tcp4", "tcp6":
		addr, _ = net.ResolveTCPAddr(c.AddressNetwork, c.AddressName)
	}
	return &plugin.ReattachConfig{Pid: int(c.Pid), Addr: addr}
}

func marshallPluginReattachConfig(c *plugin.ReattachConfig) *proto.PluginReattachInfo {
	return &proto.PluginReattachInfo{Pid: uint32(c.Pid), AddressNetwork: string(c.Addr.Network()), AddressName: c.Addr.String()}
}
