package logdaemon

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestClientRegisterAndRemoveTask(t *testing.T) {
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

	time.Sleep(1 * time.Second)

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

	if _, ok := ld.runningTasks.tasks["foo"]; !ok {
		t.Fatal("Expected task foo to be present")
	}

	client.Remove(&tInfo)
	if len(ld.runningTasks.tasks) != 0 {
		t.Fatalf("Expected number of registered tasks: %v, Actual: %v", 0, len(ld.runningTasks.tasks))
	}

	if _, ok := ld.runningTasks.tasks["foo"]; ok {
		t.Fatal("Expected task foo to be not present")
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
