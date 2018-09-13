package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/containerd/fifo"
	hclog "github.com/hashicorp/go-hclog"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/client/logmon"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "run" {
		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: logmon.Handshake,
			Plugins:         logmon.PluginMap,
			Cmd:             exec.Command("go", "run", "main.go"),
			AllowedProtocols: []plugin.Protocol{
				plugin.ProtocolGRPC,
			},
		})
		defer client.Kill()

		rpcClient, err := client.Client()
		if err != nil {
			panic(err)
		}

		raw, err := rpcClient.Dispense("logmon")
		l := raw.(logmon.LogMon)

		dir, _ := ioutil.TempDir("", "")
		fmt.Println("log: ", dir)
		err = l.Start(&logmon.LogConfig{
			LogDir:        dir,
			FifoDir:       dir,
			StdoutLogFile: "test.stdout",
			StderrLogFile: "test.stderr",
			MaxFiles:      10,
			MaxFileSizeMB: 10,
		})
		fmt.Println("called start")
		if err != nil {
			panic(err)
		}
		time.Sleep(60 * time.Second)
		l.Stop()
	} else if len(os.Args) > 2 && os.Args[1] == "write" {
		f, err := fifo.OpenFifo(context.Background(), os.Args[2], syscall.O_WRONLY|syscall.O_NONBLOCK, 0600)
		if err != nil {
			panic(err)
		}

		io.Copy(f, os.Stdin)
	} else {
		plugin.Serve(&plugin.ServeConfig{
			HandshakeConfig: logmon.Handshake,
			Plugins: map[string]plugin.Plugin{
				"logmon": logmon.NewPlugin(logmon.NewLogMon(hclog.Default().Named("logmon.test"))),
			},
			GRPCServer: plugin.DefaultGRPCServer,
		})
	}
}

func factory(log hclog.Logger) interface{} {
	return logmon.NewLogMon(log)
}
