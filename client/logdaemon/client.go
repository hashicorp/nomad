package logdaemon

import (
	"log"
	"net/rpc"
)

type LogDaemonClient struct {
	IPCAddr string
	logger  *log.Logger
}

func NewLogDaemonClient(addr string, logger *log.Logger) *LogDaemonClient {
	return &LogDaemonClient{IPCAddr: addr, logger: logger}
}

func (c *LogDaemonClient) Register(taskInfo *TaskInfo) {
	client, err := rpc.DialHTTP("tcp", c.IPCAddr)
	if err != nil {
		c.logger.Printf("[INFO] client: error dialing log daemon ipc endpoint: %v", err)
		return
	}

	var response string
	if err := client.Call("RunningTasks.Register", taskInfo, &response); err != nil {
		c.logger.Printf("[INFO] client: error registering task with log daemon: %v", err)
	}
}

func (c *LogDaemonClient) Remove(taskInfo *TaskInfo) {
	client, err := rpc.DialHTTP("tcp", c.IPCAddr)
	if err != nil {
		c.logger.Printf("[INFO] client: error dialing log daemon ipc endpoint: %v", err)
		return
	}

	var response string
	if err := client.Call("RunningTasks.Remove", taskInfo, &response); err != nil {
		c.logger.Printf("[INFO] client: error registering task with log daemon: %v", err)
	}
}
