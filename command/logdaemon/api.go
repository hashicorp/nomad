package logdaemon

import (
	"fmt"
	"net"
	"net/http"
)

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
}

// Ping responds by writing pong to the response. Serves as the health check
// endpoint for the log daemon
func (ld *LogDaemon) Ping(resp http.ResponseWriter, req *http.Request) {
	fmt.Fprint(resp, "pong")
}
