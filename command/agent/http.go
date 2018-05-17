package agent

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/rs/cors"
	"github.com/ugorji/go/codec"
)

const (
	// ErrInvalidMethod is used if the HTTP method is not supported
	ErrInvalidMethod = "Invalid method"

	// ErrEntOnly is the error returned if accessing an enterprise only
	// endpoint
	ErrEntOnly = "Nomad Enterprise only endpoint"
)

var (
	// Set to false by stub_asset if the ui build tag isn't enabled
	uiEnabled = true

	// Overridden if the ui build tag isn't enabled
	stubHTML = ""

	// allowCORS sets permissive CORS headers for a handler
	allowCORS = cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET"},
		AllowedHeaders: []string{"*"},
	})
)

// HTTPServer is used to wrap an Agent and expose it over an HTTP interface
type HTTPServer struct {
	agent      *Agent
	mux        *http.ServeMux
	listener   net.Listener
	listenerCh chan struct{}
	logger     *log.Logger
	Addr       string
}

// NewHTTPServer starts new HTTP server over the agent
func NewHTTPServer(agent *Agent, config *Config) (*HTTPServer, error) {
	// Start the listener
	lnAddr, err := net.ResolveTCPAddr("tcp", config.normalizedAddrs.HTTP)
	if err != nil {
		return nil, err
	}
	ln, err := config.Listener("tcp", lnAddr.IP.String(), lnAddr.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to start HTTP listener: %v", err)
	}

	// If TLS is enabled, wrap the listener with a TLS listener
	if config.TLSConfig.EnableHTTP {
		tlsConf := &tlsutil.Config{
			VerifyIncoming:       config.TLSConfig.VerifyHTTPSClient,
			VerifyOutgoing:       true,
			VerifyServerHostname: config.TLSConfig.VerifyServerHostname,
			CAFile:               config.TLSConfig.CAFile,
			CertFile:             config.TLSConfig.CertFile,
			KeyFile:              config.TLSConfig.KeyFile,
			KeyLoader:            config.TLSConfig.GetKeyLoader(),
		}
		tlsConfig, err := tlsConf.IncomingTLSConfig()
		if err != nil {
			return nil, err
		}
		ln = tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, tlsConfig)
	}

	// Create the mux
	mux := http.NewServeMux()

	// Create the server
	srv := &HTTPServer{
		agent:      agent,
		mux:        mux,
		listener:   ln,
		listenerCh: make(chan struct{}),
		logger:     agent.logger,
		Addr:       ln.Addr().String(),
	}
	srv.registerHandlers(config.EnableDebug)

	// Handle requests with gzip compression
	gzip, err := gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(0))
	if err != nil {
		return nil, err
	}

	go func() {
		defer close(srv.listenerCh)
		http.Serve(ln, gzip(mux))
	}()

	return srv, nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by NewHttpServer so
// dead TCP connections eventually go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(30 * time.Second)
	return tc, nil
}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() {
	if s != nil {
		s.logger.Printf("[DEBUG] http: Shutting down http server")
		s.listener.Close()
		<-s.listenerCh // block until http.Serve has returned.
	}
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	s.mux.HandleFunc("/v1/jobs", s.wrap(s.JobsRequest))
	s.mux.HandleFunc("/v1/jobs/parse", s.wrap(s.JobsParseRequest))
	s.mux.HandleFunc("/v1/job/", s.wrap(s.JobSpecificRequest))

	s.mux.HandleFunc("/v1/nodes", s.wrap(s.NodesRequest))
	s.mux.HandleFunc("/v1/node/", s.wrap(s.NodeSpecificRequest))

	s.mux.HandleFunc("/v1/allocations", s.wrap(s.AllocsRequest))
	s.mux.HandleFunc("/v1/allocation/", s.wrap(s.AllocSpecificRequest))

	s.mux.HandleFunc("/v1/evaluations", s.wrap(s.EvalsRequest))
	s.mux.HandleFunc("/v1/evaluation/", s.wrap(s.EvalSpecificRequest))

	s.mux.HandleFunc("/v1/deployments", s.wrap(s.DeploymentsRequest))
	s.mux.HandleFunc("/v1/deployment/", s.wrap(s.DeploymentSpecificRequest))

	s.mux.HandleFunc("/v1/acl/policies", s.wrap(s.ACLPoliciesRequest))
	s.mux.HandleFunc("/v1/acl/policy/", s.wrap(s.ACLPolicySpecificRequest))

	s.mux.HandleFunc("/v1/acl/bootstrap", s.wrap(s.ACLTokenBootstrap))
	s.mux.HandleFunc("/v1/acl/tokens", s.wrap(s.ACLTokensRequest))
	s.mux.HandleFunc("/v1/acl/token", s.wrap(s.ACLTokenSpecificRequest))
	s.mux.HandleFunc("/v1/acl/token/", s.wrap(s.ACLTokenSpecificRequest))

	s.mux.Handle("/v1/client/fs/", wrapCORS(s.wrap(s.FsRequest)))
	s.mux.HandleFunc("/v1/client/gc", s.wrap(s.ClientGCRequest))
	s.mux.Handle("/v1/client/stats", wrapCORS(s.wrap(s.ClientStatsRequest)))
	s.mux.Handle("/v1/client/allocation/", wrapCORS(s.wrap(s.ClientAllocRequest)))

	s.mux.HandleFunc("/v1/agent/self", s.wrap(s.AgentSelfRequest))
	s.mux.HandleFunc("/v1/agent/join", s.wrap(s.AgentJoinRequest))
	s.mux.HandleFunc("/v1/agent/members", s.wrap(s.AgentMembersRequest))
	s.mux.HandleFunc("/v1/agent/force-leave", s.wrap(s.AgentForceLeaveRequest))
	s.mux.HandleFunc("/v1/agent/servers", s.wrap(s.AgentServersRequest))
	s.mux.HandleFunc("/v1/agent/keyring/", s.wrap(s.KeyringOperationRequest))
	s.mux.HandleFunc("/v1/agent/health", s.wrap(s.HealthRequest))

	s.mux.HandleFunc("/v1/metrics", s.wrap(s.MetricsRequest))

	s.mux.HandleFunc("/v1/validate/job", s.wrap(s.ValidateJobRequest))

	s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))

	s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeaderRequest))
	s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeersRequest))

	s.mux.HandleFunc("/v1/search", s.wrap(s.SearchRequest))

	s.mux.HandleFunc("/v1/operator/raft/", s.wrap(s.OperatorRequest))
	s.mux.HandleFunc("/v1/operator/autopilot/configuration", s.wrap(s.OperatorAutopilotConfiguration))
	s.mux.HandleFunc("/v1/operator/autopilot/health", s.wrap(s.OperatorServerHealth))

	s.mux.HandleFunc("/v1/system/gc", s.wrap(s.GarbageCollectRequest))
	s.mux.HandleFunc("/v1/system/reconcile/summaries", s.wrap(s.ReconcileJobSummaries))

	if uiEnabled {
		s.mux.Handle("/ui/", http.StripPrefix("/ui/", handleUI(http.FileServer(&UIAssetWrapper{FileSystem: assetFS()}))))
	} else {
		// Write the stubHTML
		s.mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(stubHTML))
		})
	}
	s.mux.Handle("/", handleRootRedirect())

	if enableDebug {
		s.mux.HandleFunc("/debug/pprof/", pprof.Index)
		s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		s.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	// Register enterprise endpoints.
	s.registerEnterpriseHandlers()
}

// HTTPCodedError is used to provide the HTTP error code
type HTTPCodedError interface {
	error
	Code() int
}

type UIAssetWrapper struct {
	FileSystem *assetfs.AssetFS
}

func (fs *UIAssetWrapper) Open(name string) (http.File, error) {
	if file, err := fs.FileSystem.Open(name); err == nil {
		return file, nil
	} else {
		// serve index.html instead of 404ing
		if err == os.ErrNotExist {
			return fs.FileSystem.Open("index.html")
		}
		return nil, err
	}
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

func handleUI(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		header := w.Header()
		header.Add("Content-Security-Policy", "default-src 'none'; connect-src *; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'")
		h.ServeHTTP(w, req)
		return
	})
}

func handleRootRedirect() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", 307)
		return
	})
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPAPIResponseHeaders)
		// Invoke the handler
		reqURL := req.URL.String()
		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] http: Request %v %v (%v)", req.Method, reqURL, time.Now().Sub(start))
		}()
		obj, err := handler(resp, req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			s.logger.Printf("[ERR] http: Request %v, error: %v", reqURL, err)
			code := 500
			errMsg := err.Error()
			if http, ok := err.(HTTPCodedError); ok {
				code = http.Code()
			} else {
				// RPC errors get wrapped, so manually unwrap by only looking at their suffix
				if strings.HasSuffix(errMsg, structs.ErrPermissionDenied.Error()) {
					errMsg = structs.ErrPermissionDenied.Error()
					code = 403
				} else if strings.HasSuffix(errMsg, structs.ErrTokenNotFound.Error()) {
					errMsg = structs.ErrTokenNotFound.Error()
					code = 403
				}
			}

			resp.WriteHeader(code)
			resp.Write([]byte(errMsg))
			return
		}

		prettyPrint := false
		if v, ok := req.URL.Query()["pretty"]; ok {
			if len(v) > 0 && (len(v[0]) == 0 || v[0] != "0") {
				prettyPrint = true
			}
		}

		// Write out the JSON object
		if obj != nil {
			var buf bytes.Buffer
			if prettyPrint {
				enc := codec.NewEncoder(&buf, structs.JsonHandlePretty)
				err = enc.Encode(obj)
				if err == nil {
					buf.Write([]byte("\n"))
				}
			} else {
				enc := codec.NewEncoder(&buf, structs.JsonHandle)
				err = enc.Encode(obj)
			}
			if err != nil {
				goto HAS_ERR
			}
			resp.Header().Set("Content-Type", "application/json")
			resp.Write(buf.Bytes())
		}
	}
	return f
}

// decodeBody is used to decode a JSON request body
func decodeBody(req *http.Request, out interface{}) error {
	dec := json.NewDecoder(req.Body)
	return dec.Decode(&out)
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

// setHeaders is used to set canonical response header fields
func setHeaders(resp http.ResponseWriter, headers map[string]string) {
	for field, value := range headers {
		resp.Header().Set(http.CanonicalHeaderKey(field), value)
	}
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

// parsePrefix is used to parse the ?prefix query param
func parsePrefix(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	if prefix := query.Get("prefix"); prefix != "" {
		b.Prefix = prefix
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

// parseNamespace is used to parse the ?namespace parameter
func parseNamespace(req *http.Request, n *string) {
	if other := req.URL.Query().Get("namespace"); other != "" {
		*n = other
	} else if *n == "" {
		*n = structs.DefaultNamespace
	}
}

// parseToken is used to parse the X-Nomad-Token param
func (s *HTTPServer) parseToken(req *http.Request, token *string) {
	if other := req.Header.Get("X-Nomad-Token"); other != "" {
		*token = other
		return
	}
}

// parse is a convenience method for endpoints that need to parse multiple flags
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, r *string, b *structs.QueryOptions) bool {
	s.parseRegion(req, r)
	s.parseToken(req, &b.AuthToken)
	parseConsistency(req, b)
	parsePrefix(req, b)
	parseNamespace(req, &b.Namespace)
	return parseWait(resp, req, b)
}

// parseWriteRequest is a convenience method for endpoints that need to parse a
// write request.
func (s *HTTPServer) parseWriteRequest(req *http.Request, w *structs.WriteRequest) {
	parseNamespace(req, &w.Namespace)
	s.parseToken(req, &w.AuthToken)
	s.parseRegion(req, &w.Region)
}

// wrapCORS wraps a HandlerFunc in allowCORS and returns a http.Handler
func wrapCORS(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return allowCORS.Handler(http.HandlerFunc(f))
}
