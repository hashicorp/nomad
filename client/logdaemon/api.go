package logdaemon

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"os/signal"
	"strings"

	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/client/config"
)

type TaskInfo struct {
	HandleId string
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
	key := fmt.Sprintf("%s-%s", task.AllocID, task.Name)
	r.tasks[key] = task
	return nil
}

func (r *RunningTasks) Remove(task *TaskInfo, reply *string) error {
	r.logger.Printf("[DEBUG] client.logdaemon: de-registering task: %v", task.Name)
	key := fmt.Sprintf("%s-%s", task.AllocID, task.Name)
	delete(r.tasks, key)
	return nil
}

type LogDaemon struct {
	mux          *http.ServeMux
	apiListener  net.Listener
	ipcListener  net.Listener
	runningTasks *RunningTasks
	config       *config.Config

	logger *log.Logger
}

// NewLogDaemon creates a new logging daemon
func NewLogDaemon(config *config.Config) (*LogDaemon, error) {

	// Create the mux for api
	mux := http.NewServeMux()

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
		mux:         mux,
		apiListener: apiListener,
		ipcListener: ipcListener,
		runningTasks: &RunningTasks{
			tasks:  make(map[string]*TaskInfo),
			logger: logger,
		},
		logger: logger,
	}

	// Configure the routes
	ld.configureRoutes()

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
	go http.Serve(ld.apiListener, ld.mux)

	rpc.HandleHTTP()
	ld.logger.Printf("[INFO] client.logdaemon: ipc server has started, it is listening on %v", ld.ipcListener.Addr())
	go http.Serve(ld.ipcListener, nil)
	return nil
}

// configureRoutes sets up the mux with the various api end points of the log
// daemon
func (ld *LogDaemon) configureRoutes() {
	ld.mux.HandleFunc("/ping", ld.Ping)
	ld.mux.HandleFunc("/v1/logs/", ld.StreamLogs)

	rpc.Register(ld.runningTasks)
	rpc.Register(ld)
}

// Ping responds by writing pong to the response. Serves as the health check
// endpoint for the log daemon
func (ld *LogDaemon) Ping(resp http.ResponseWriter, req *http.Request) {
	fmt.Fprint(resp, "pong")
}

func (ld *LogDaemon) StreamLogs(resp http.ResponseWriter, req *http.Request) {
	urlTokens := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
	if len(urlTokens) < 4 {
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprint(resp, "alloc id and task names are mandatory")
		return
	}

	if len(urlTokens) > 5 {
		resp.WriteHeader(http.StatusNotFound)
		return
	}
	//allocId := urlTokens[2]
	//taskName := urlTokens[3]

	if len(urlTokens) == 4 {
		fmt.Fprint(resp, "multiplexed")
		return
	}

	if len(urlTokens) == 5 {
		streamName := urlTokens[5]
		if streamName != "stdout" || streamName != "stderr" {
			resp.WriteHeader(http.StatusNotAcceptable)
			fmt.Fprint(resp, "only stdout and stderr can be streamed")
			return
		}
		fmt.Fprint(resp, streamName)
	}

	fmt.Fprint(resp, req.URL.Path)
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
