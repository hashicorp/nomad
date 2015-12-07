package logdaemon

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
)

type trackedTask struct {
	Name    string `json:"name"`
	AllocId string `json:"alloc"`
	Driver  string `json:"driver"`
}

func (tt *trackedTask) Hash() string {
	h := sha1.New()
	io.WriteString(h, tt.Name)
	io.WriteString(h, tt.AllocId)
	return fmt.Sprintf("%x", h.Sum(nil))
}

type LogDaemonConfig struct {
	Port      int
	Interface string
}

func NewLogDaemonConfig() *LogDaemonConfig {
	return &LogDaemonConfig{
		Port:      4470,
		Interface: "127.0.0.1",
	}
}

type LogDaemon struct {
	mux      *http.ServeMux
	listener net.Listener
	tasks    map[string]*trackedTask
}

// NewLogDaemon creates a new logging daemon
func NewLogDaemon(config *LogDaemonConfig) (*LogDaemon, error) {

	// Create the mux
	mux := http.NewServeMux()

	// Create the listener
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.Interface, config.Port))
	if err != nil {
		return nil, err
	}

	// Create the log Daemon
	ld := LogDaemon{
		mux:      mux,
		listener: listener,
		tasks:    make(map[string]*trackedTask),
	}

	// Configure the routes
	ld.configureRoutes()

	return &ld, nil
}

// Start starts the http server of the log daemon
func (ld *LogDaemon) Start() error {
	return http.Serve(ld.listener, ld.mux)
}

// configureRoutes sets up the mux with the various api end points of the log
// daemon
func (ld *LogDaemon) configureRoutes() {
	ld.mux.HandleFunc("/ping", ld.Ping)
	ld.mux.HandleFunc("/internal/tasks", ld.Tasks)
}

// Ping responds by writing pong to the response. Serves as the health check
// endpoint for the log daemon
func (ld *LogDaemon) Ping(resp http.ResponseWriter, req *http.Request) {
	fmt.Fprint(resp, "pong")
}

// Tasks handles requests for registering new tasks or deleting tasks with the
// logging daemon. Once a task is registered the logging daemon can stream logs
// produces by the task.
func (ld *LogDaemon) Tasks(resp http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		ld.registerTask(resp, req)
	} else if req.Method == "DELETE" {
		ld.deleteTask(resp, req)
	} else {
		resp.WriteHeader(http.StatusBadRequest)
	}
}

func (ld *LogDaemon) registerTask(resp http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var tt trackedTask
	if err := decoder.Decode(&tt); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Error while decoding payload for register task req: %v", err)
		return
	}
	ld.tasks[tt.Hash()] = &trackedTask{Name: tt.Name, AllocId: tt.AllocId, Driver: tt.Driver}
	resp.WriteHeader(http.StatusCreated)
}

func (ld *LogDaemon) deleteTask(resp http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var tt trackedTask
	if err := decoder.Decode(&tt); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Error while decoding payload for delete task req: %v", err)
		return
	}
	delete(ld.tasks, tt.Hash())
	resp.WriteHeader(http.StatusAccepted)
}
