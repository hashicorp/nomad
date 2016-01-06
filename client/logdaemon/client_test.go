package logdaemon

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestClientRegisterTask(t *testing.T) {
	apiPort, err := getFreePort()
	if err != nil {
		t.Fatalf("error in getting free port : %v", err)
	}
	ipcPort, err := getFreePort()
	if err != nil {
		t.Fatalf("error in getting free port: %v", err)
	}

	cfg := structs.LogDaemonConfig{
		APIAddr: fmt.Sprintf("127.0.0.1:%v", apiPort),
		IPCAddr: fmt.Sprintf("127.0.0.1:%v", ipcPort),
	}

	ld, err := NewLogDaemon(&cfg)

	if err != nil {
		t.Fatalf("error in starting log daemon: %v", err)
	}

	ld.Start()

	time.Sleep(2 * time.Second)

	client := NewLogDaemonClient(cfg.IPCAddr, ld.logger)

	tInfo := TaskInfo{
		Name:     "foo",
		HandleId: "bar",
		AllocID:  "baz",
	}

	client.Register(&tInfo)

	if len(ld.runningTasks.tasks) != 1 {
		t.Fatalf("Expected number of registered tasks: %v, Actual: %v", 1, len(ld.runningTasks.tasks))
	}
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	dummyServer, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}

	defer dummyServer.Close()
	return dummyServer.Addr().(*net.TCPAddr).Port, nil
}
