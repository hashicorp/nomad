package logdaemon

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"

	"github.com/julienschmidt/httprouter"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/driver"
)

type TaskInfo struct {
	HandleID string
	AllocDir *allocdir.AllocDir
	AllocID  string
	Name     string
	Driver   string
}

type RunningTasks struct {
	tasks  map[string]*TaskInfo
	logger *log.Logger
}

func (r *RunningTasks) Register(task *TaskInfo, reply *string) error {
	r.logger.Printf("[DEBUG] client.logdaemon: registering task: %v", task.Name)
	key := taskId(task.AllocID, task.Name)
	r.tasks[key] = task
	return nil
}

func (r *RunningTasks) Remove(task *TaskInfo, reply *string) error {
	r.logger.Printf("[DEBUG] client.logdaemon: de-registering task: %v", task.Name)
	key := taskId(task.AllocID, task.Name)
	delete(r.tasks, key)
	return nil
}

type LogDaemon struct {
	router       *httprouter.Router
	apiListener  net.Listener
	ipcListener  net.Listener
	runningTasks *RunningTasks
	config       *config.Config

	logger *log.Logger
}

// NewLogDaemon creates a new logging daemon
func NewLogDaemon(config *config.Config) (*LogDaemon, error) {
	// Create the api listener
	apiListener, err := net.Listen("tcp", config.Node.LogDaemonAddr)
	if err != nil {
		return nil, err
	}

	// Create the ipc listener
	ipcListener, err := net.Listen("tcp", config.LogDaemonIPCAddr)
	if err != nil {
		return nil, err
	}
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Create the log Daemon
	ld := LogDaemon{
		router:      httprouter.New(),
		apiListener: apiListener,
		ipcListener: ipcListener,
		runningTasks: &RunningTasks{
			tasks:  make(map[string]*TaskInfo),
			logger: logger,
		},
		config: config,
		logger: logger,
	}

	// Configure the routes
	ld.configureRoutes()
	ld.router.RedirectTrailingSlash = true

	return &ld, nil
}

func (ld *LogDaemon) SetConfig(config *config.Config, reply *string) error {
	ld.logger.Printf("[INFO] client.logdaemon: setting config")
	ld.config = config
	return nil
}

// Start starts the http server of the log daemon
func (ld *LogDaemon) Start() error {
	ld.logger.Printf("[INFO] client.logdaemon: api server has started, it is listening on %v", ld.apiListener.Addr())
	go http.Serve(ld.apiListener, ld.router)

	rpc.HandleHTTP()
	ld.logger.Printf("[INFO] client.logdaemon: ipc server has started, it is listening on %v", ld.ipcListener.Addr())
	go http.Serve(ld.ipcListener, nil)
	return nil
}

// configureRoutes sets up the mux with the various api end points of the log
// daemon
func (ld *LogDaemon) configureRoutes() {
	ld.router.GET("/ping", ld.Ping)
	ld.router.GET("/v1/logs/:allocation/:task", ld.MuxLogs)
	ld.router.GET("/v1/logs/:allocation/:task/stdout", ld.Stdout)
	ld.router.GET("/v1/logs/:allocation/:task/stderr", ld.Stderr)

	rpc.Register(ld.runningTasks)
	rpc.Register(ld)
}

// Ping responds by writing pong to the response. Serves as the health check
// endpoint for the log daemon
func (ld *LogDaemon) Ping(resp http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	fmt.Fprint(resp, "pong")
}

func (ld *LogDaemon) MuxLogs(resp http.ResponseWriter, req *http.Request, p httprouter.Params) {
	allocID := p.ByName("allocation")
	taskName := p.ByName("task")
	taskInfo, ok := ld.runningTasks.tasks[taskId(allocID, taskName)]
	if !ok {
		resp.WriteHeader(http.StatusNotAcceptable)
		fmt.Fprintf(resp, "task with name: %s and alloc id: %s is not running on this node", taskName, allocID)
		return
	}
	handle, err := ld.driverHandle(taskInfo)
	if err != nil {
		ld.logger.Printf("[ERROR] client.logdaemon: could not create driver handle: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	reader, err := handle.Logs()
	if err != nil {
		ld.logger.Printf("[ERROR] client.logdaemon: error reading logs: %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	io.Copy(resp, reader)
	return
}

func (ld *LogDaemon) Stdout(resp http.ResponseWriter, req *http.Request, p httprouter.Params) {
}

func (ld *LogDaemon) Stderr(resp http.ResponseWriter, req *http.Request, p httprouter.Params) {
}

func (ld *LogDaemon) Wait() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	for {
		select {
		case <-signalChan:
			ld.logger.Printf("[INFO] client.logdaemon: shutting down")
			os.Exit(0)
		}
	}
}

// createDriver makes a driver for the task
func (ld *LogDaemon) driverHandle(taskInfo *TaskInfo) (driver.DriverHandle, error) {
	driverCtx := driver.NewDriverContext(taskInfo.Name, ld.config, ld.config.Node, ld.logger)
	d, err := driver.NewDriver(taskInfo.Driver, driverCtx)
	if err != nil {
		ld.logger.Printf("[ERROR] client.logdaemon: failed to create driver '%s' for alloc %s: %v",
			taskInfo.Driver, taskInfo.AllocID, err)
		return nil, err
	}
	ctx := driver.NewExecContext(taskInfo.AllocDir, taskInfo.AllocID)
	handle, err := d.Open(ctx, taskInfo.HandleID)
	if err != nil {
		ld.logger.Printf("[ERROR] client.logdaemon: could not create driver handle: %v", err)
		return nil, err
	}

	return handle, nil
}

func taskId(allocID string, taskName string) string {
	return fmt.Sprintf("%s-%s", allocID, taskName)
}
