package logdaemon

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
)

type TaskInfo struct {
	HandleId string
	AllocDir *allocdir.AllocDir
	AllocID  string
	Name     string
}

type RunningTasks struct {
	tasks map[string]*TaskInfo
}

func (r *RunningTasks) Register(task *TaskInfo, reply *string) error {
	r.tasks[task.Name] = task
	return nil
}

func (r *RunningTasks) Remove(task *TaskInfo, reply *string) error {
	delete(r.tasks, task.Name)
	return nil
}

type LogDaemon struct {
	mux          *http.ServeMux
	apiListener  net.Listener
	ipcListener  net.Listener
	runningTasks *RunningTasks

	logger *log.Logger
}

// NewLogDaemon creates a new logging daemon
func NewLogDaemon(config *structs.LogDaemonConfig) (*LogDaemon, error) {

	// Create the mux for api
	mux := http.NewServeMux()

	// Create the api listener
	apiListener, err := net.Listen("tcp", config.APIAddr)
	if err != nil {
		return nil, err
	}

	// Create the ipc listener
	ipcListener, err := net.Listen("tcp", config.RPCAddr)
	if err != nil {
		return nil, err
	}

	// Create the log Daemon
	ld := LogDaemon{
		mux:         mux,
		apiListener: apiListener,
		ipcListener: ipcListener,
		runningTasks: &RunningTasks{
			tasks: make(map[string]*TaskInfo),
		},
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}

	// Configure the routes
	ld.configureRoutes()

	go ld.startIpcServer()

	return &ld, nil
}

// Start starts the http server of the log daemon
func (ld *LogDaemon) Start() error {
	ld.logger.Printf("[INFO] client.logdaemon: api server has started, it is listening on %v", ld.apiListener.Addr())
	return http.Serve(ld.apiListener, ld.mux)
}

// configureRoutes sets up the mux with the various api end points of the log
// daemon
func (ld *LogDaemon) configureRoutes() {
	ld.mux.HandleFunc("/ping", ld.Ping)
}

func (ld *LogDaemon) startIpcServer() {
	rpc.Register(ld.runningTasks)
	ld.logger.Printf("[INFO] client.logdaemon: ipc server has started, it is listening on %v", ld.ipcListener.Addr())
	rpc.Accept(ld.ipcListener)
}

// Ping responds by writing pong to the response. Serves as the health check
// endpoint for the log daemon
func (ld *LogDaemon) Ping(resp http.ResponseWriter, req *http.Request) {
	fmt.Fprint(resp, "pong")
}
