package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/mitchellh/mapstructure"
)

const (
	// ErrInvalidMethod is used if the HTTP method is not supported
	ErrInvalidMethod = "Invalid method"
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
	s.mux.HandleFunc("/v1/jobs", s.wrap(s.JobsRequest))
	s.mux.HandleFunc("/v1/job/", s.wrap(s.JobSpecificRequest))

	s.mux.HandleFunc("/v1/nodes", s.wrap(s.NodesRequest))
	s.mux.HandleFunc("/v1/node/", s.wrap(s.NodeSpecificRequest))

	s.mux.HandleFunc("/v1/allocations", s.wrap(s.AllocsRequest))
	s.mux.HandleFunc("/v1/allocation/", s.wrap(s.AllocSpecificRequest))

	s.mux.HandleFunc("/v1/evaluations", s.wrap(s.EvalsRequest))
	s.mux.HandleFunc("/v1/evaluation/", s.wrap(s.EvalSpecificRequest))

	s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeaderRequest))
	s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeersRequest))

	if enableDebug {
		s.mux.HandleFunc("/debug/pprof/", pprof.Index)
		s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}
}

// HTTPCodedError is used to provide the HTTP error code
type HTTPCodedError interface {
	error
	Code() int
}

func CodedError(c int, s string) HTTPCodedError {
	return &codedError{s, c}
}

type codedError struct {
	s    string
	code int
}

func (e *codedError) Error() string {
	return e.s
}

func (e *codedError) Code() int {
	return e.code
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		// Invoke the handler
		reqURL := req.URL.String()
		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] http: Request %v (%v)", reqURL, time.Now().Sub(start))
		}()
		obj, err := handler(resp, req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			s.logger.Printf("[ERR] http: Request %v, error: %v", reqURL, err)
			code := 500
			if http, ok := err.(HTTPCodedError); ok {
				code = http.Code()
			}
			resp.WriteHeader(code)
			resp.Write([]byte(err.Error()))
			return
		}

		prettyPrint := false
		if _, ok := req.URL.Query()["pretty"]; ok {
			prettyPrint = true
		}

		// Write out the JSON object
		if obj != nil {
			var buf []byte
			if prettyPrint {
				buf, err = json.MarshalIndent(obj, "", "    ")
			} else {
				buf, err = json.Marshal(obj)
			}
			if err != nil {
				goto HAS_ERR
			}
			resp.Header().Set("Content-Type", "application/json")
			resp.Write(buf)
		}
	}
	return f
}

// decodeBody is used to decode a JSON request body
func decodeBody(req *http.Request, out interface{}, cb func(interface{}) error) error {
	var raw interface{}
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&raw); err != nil {
		return err
	}

	// Invoke the callback prior to decode
	if cb != nil {
		if err := cb(raw); err != nil {
			return err
		}
	}
	return mapstructure.Decode(raw, out)
}

// setIndex is used to set the index response header
func setIndex(resp http.ResponseWriter, index uint64) {
	resp.Header().Set("X-Nomad-Index", strconv.FormatUint(index, 10))
}

// setKnownLeader is used to set the known leader header
func setKnownLeader(resp http.ResponseWriter, known bool) {
	s := "true"
	if !known {
		s = "false"
	}
	resp.Header().Set("X-Nomad-KnownLeader", s)
}

// setLastContact is used to set the last contact header
func setLastContact(resp http.ResponseWriter, last time.Duration) {
	lastMsec := uint64(last / time.Millisecond)
	resp.Header().Set("X-Nomad-LastContact", strconv.FormatUint(lastMsec, 10))
}

// setMeta is used to set the query response meta data
func setMeta(resp http.ResponseWriter, m *structs.QueryMeta) {
	setIndex(resp, m.Index)
	setLastContact(resp, m.LastContact)
	setKnownLeader(resp, m.KnownLeader)
}

// parseWait is used to parse the ?wait and ?index query params
// Returns true on error
func parseWait(resp http.ResponseWriter, req *http.Request, b *structs.QueryOptions) bool {
	query := req.URL.Query()
	if wait := query.Get("wait"); wait != "" {
		dur, err := time.ParseDuration(wait)
		if err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte("Invalid wait time"))
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte("Invalid index"))
			return true
		}
		b.MinQueryIndex = index
	}
	return false
}

// parseConsistency is used to parse the ?stale query params.
func parseConsistency(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	if _, ok := query["stale"]; ok {
		b.AllowStale = true
	}
}

// parseRegion is used to parse the ?region query param
func (s *HTTPServer) parseRegion(req *http.Request, r *string) {
	if other := req.URL.Query().Get("region"); other != "" {
		*r = other
	} else if *r == "" {
		*r = s.agent.config.Region
	}
}

// parse is a convenience method for endpoints that need to parse multiple flags
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, r *string, b *structs.QueryOptions) bool {
	s.parseRegion(req, r)
	parseConsistency(req, b)
	return parseWait(resp, req, b)
}
