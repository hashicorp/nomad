package agent

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
)

// HTTPServer is used to wrap an Agent and expose it over an HTTP interface
type HTTPServer struct {
	agent    *Agent
	mux      *http.ServeMux
	listener net.Listener
	logger   *log.Logger
}

// NewHTTPServer starts new HTTP server over the agent
func NewHTTPServer(agent *Agent, config *Config, logOutput io.Writer) (*HTTPServer, error) {
	// Start the listener
	ln, err := net.Listen("tcp", config.HttpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to start HTTP listener on %s: %v", config.HttpAddr, err)
	}

	// Create the mux
	mux := http.NewServeMux()

	// Create the server
	srv := &HTTPServer{
		agent:    agent,
		mux:      mux,
		listener: ln,
		logger:   log.New(logOutput, "", log.LstdFlags),
	}
	srv.registerHandlers(config.EnableDebug)

	// Start the server
	go http.Serve(ln, mux)
	return srv, nil
}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() {
	if s != nil {
		s.logger.Printf("[DEBUG] http: Shutting down http server")
		s.listener.Close()
	}
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	if enableDebug {
		s.mux.HandleFunc("/debug/pprof/", pprof.Index)
		s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}
}
