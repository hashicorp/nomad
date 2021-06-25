package agent

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-connlimit"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/hashicorp/nomad/helper/noxssrw"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// ErrInvalidMethod is used if the HTTP method is not supported
	ErrInvalidMethod = "Invalid method"

	// ErrEntOnly is the error returned if accessing an enterprise only
	// endpoint
	ErrEntOnly = "Nomad Enterprise only endpoint"

	// ContextKeyReqID is a unique ID for a given request
	ContextKeyReqID = "requestID"

	// MissingRequestID is a placeholder if we cannot retrieve a request
	// UUID from context
	MissingRequestID = "<missing request id>"
)

var (
	// Set to false by stub_asset if the ui build tag isn't enabled
	uiEnabled = true

	// Overridden if the ui build tag isn't enabled
	stubHTML = ""

	// allowCORS sets permissive CORS headers for a handler
	allowCORS = cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"HEAD", "GET"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
)

type handlerFn func(resp http.ResponseWriter, req *http.Request) (interface{}, error)
type handlerByteFn func(resp http.ResponseWriter, req *http.Request) ([]byte, error)

// HTTPServer is used to wrap an Agent and expose it over an HTTP interface
type HTTPServer struct {
	agent      *Agent
	mux        *http.ServeMux
	listener   net.Listener
	listenerCh chan struct{}
	logger     log.Logger
	Addr       string

	wsUpgrader *websocket.Upgrader
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
		tlsConf, err := tlsutil.NewTLSConfiguration(config.TLSConfig, config.TLSConfig.VerifyHTTPSClient, true)
		if err != nil {
			return nil, err
		}

		tlsConfig, err := tlsConf.IncomingTLSConfig()
		if err != nil {
			return nil, err
		}
		ln = tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, tlsConfig)
	}

	// Create the mux
	mux := http.NewServeMux()

	wsUpgrader := &websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
	}

	// Create the server
	srv := &HTTPServer{
		agent:      agent,
		mux:        mux,
		listener:   ln,
		listenerCh: make(chan struct{}),
		logger:     agent.httpLogger,
		Addr:       ln.Addr().String(),
		wsUpgrader: wsUpgrader,
	}
	srv.registerHandlers(config.EnableDebug)

	// Handle requests with gzip compression
	gzip, err := gziphandler.GzipHandlerWithOpts(gziphandler.MinSize(0))
	if err != nil {
		return nil, err
	}

	// Get connection handshake timeout limit
	handshakeTimeout, err := time.ParseDuration(config.Limits.HTTPSHandshakeTimeout)
	if err != nil {
		return nil, fmt.Errorf("error parsing https_handshake_timeout: %v", err)
	} else if handshakeTimeout < 0 {
		return nil, fmt.Errorf("https_handshake_timeout must be >= 0")
	}

	// Get max connection limit
	maxConns := 0
	if mc := config.Limits.HTTPMaxConnsPerClient; mc != nil {
		maxConns = *mc
	}
	if maxConns < 0 {
		return nil, fmt.Errorf("http_max_conns_per_client must be >= 0")
	}

	// Create HTTP server with timeouts
	httpServer := http.Server{
		Addr:      srv.Addr,
		Handler:   gzip(mux),
		ConnState: makeConnState(config.TLSConfig.EnableHTTP, handshakeTimeout, maxConns),
		ErrorLog:  newHTTPServerLogger(srv.logger),
	}

	go func() {
		defer close(srv.listenerCh)
		httpServer.Serve(ln)
	}()

	return srv, nil
}

// makeConnState returns a ConnState func for use in an http.Server. If
// isTLS=true and handshakeTimeout>0 then the handshakeTimeout will be applied
// as a connection deadline to new connections and removed when the connection
// is active (meaning it has successfully completed the TLS handshake).
//
// If limit > 0, a per-address connection limit will be enabled regardless of
// TLS. If connLimit == 0 there is no connection limit.
func makeConnState(isTLS bool, handshakeTimeout time.Duration, connLimit int) func(conn net.Conn, state http.ConnState) {
	if !isTLS || handshakeTimeout == 0 {
		if connLimit > 0 {
			// Still return the connection limiter
			return connlimit.NewLimiter(connlimit.Config{
				MaxConnsPerClientIP: connLimit,
			}).HTTPConnStateFunc()
		}
		return nil
	}

	if connLimit > 0 {
		// Return conn state callback with connection limiting and a
		// handshake timeout.
		connLimiter := connlimit.NewLimiter(connlimit.Config{
			MaxConnsPerClientIP: connLimit,
		}).HTTPConnStateFunc()

		return func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				// Set deadline to prevent slow send before TLS handshake or first
				// byte of request.
				conn.SetDeadline(time.Now().Add(handshakeTimeout))
			case http.StateActive:
				// Clear read deadline. We should maybe set read timeouts more
				// generally but that's a bigger task as some HTTP endpoints may
				// stream large requests and responses (e.g. snapshot) so we can't
				// set sensible blanket timeouts here.
				conn.SetDeadline(time.Time{})
			}

			// Call connection limiter
			connLimiter(conn, state)
		}
	}

	// Return conn state callback with just a handshake timeout
	// (connection limiting disabled).
	return func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			// Set deadline to prevent slow send before TLS handshake or first
			// byte of request.
			conn.SetDeadline(time.Now().Add(handshakeTimeout))
		case http.StateActive:
			// Clear read deadline. We should maybe set read timeouts more
			// generally but that's a bigger task as some HTTP endpoints may
			// stream large requests and responses (e.g. snapshot) so we can't
			// set sensible blanket timeouts here.
			conn.SetDeadline(time.Time{})
		}
	}
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
		s.logger.Debug("shutting down http server")
		s.listener.Close()
		<-s.listenerCh // block until http.Serve has returned.
	}
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	s.mux.Handle("/v1/jobs", s.wrapOpenTelemetry(s.JobsRequest))
	s.mux.Handle("/v1/jobs/parse", s.wrapOpenTelemetry(s.JobsParseRequest))
	s.mux.Handle("/v1/job/", s.wrapOpenTelemetry(s.JobSpecificRequest))

	s.mux.Handle("/v1/nodes", s.wrapOpenTelemetry(s.NodesRequest))
	s.mux.Handle("/v1/node/", s.wrapOpenTelemetry(s.NodeSpecificRequest))

	s.mux.Handle("/v1/allocations", s.wrapOpenTelemetry(s.AllocsRequest))
	s.mux.Handle("/v1/allocation/", s.wrapOpenTelemetry(s.AllocSpecificRequest))

	s.mux.Handle("/v1/evaluations", s.wrapOpenTelemetry(s.EvalsRequest))
	s.mux.Handle("/v1/evaluation/", s.wrapOpenTelemetry(s.EvalSpecificRequest))

	s.mux.Handle("/v1/deployments", s.wrapOpenTelemetry(s.DeploymentsRequest))
	s.mux.Handle("/v1/deployment/", s.wrapOpenTelemetry(s.DeploymentSpecificRequest))

	s.mux.Handle("/v1/volumes", s.wrapOpenTelemetry(s.CSIVolumesRequest))
	s.mux.Handle("/v1/volumes/external", s.wrapOpenTelemetry(s.CSIExternalVolumesRequest))
	s.mux.Handle("/v1/volumes/snapshot", s.wrapOpenTelemetry(s.CSISnapshotsRequest))
	s.mux.Handle("/v1/volume/csi/", s.wrapOpenTelemetry(s.CSIVolumeSpecificRequest))
	s.mux.Handle("/v1/plugins", s.wrapOpenTelemetry(s.CSIPluginsRequest))
	s.mux.Handle("/v1/plugin/csi/", s.wrapOpenTelemetry(s.CSIPluginSpecificRequest))

	s.mux.Handle("/v1/acl/policies", s.wrapOpenTelemetry(s.ACLPoliciesRequest))
	s.mux.Handle("/v1/acl/policy/", s.wrapOpenTelemetry(s.ACLPolicySpecificRequest))

	s.mux.Handle("/v1/acl/token/onetime", s.wrapOpenTelemetry(s.UpsertOneTimeToken))
	s.mux.Handle("/v1/acl/token/onetime/exchange", s.wrapOpenTelemetry(s.ExchangeOneTimeToken))
	s.mux.Handle("/v1/acl/bootstrap", s.wrapOpenTelemetry(s.ACLTokenBootstrap))
	s.mux.Handle("/v1/acl/tokens", s.wrapOpenTelemetry(s.ACLTokensRequest))
	s.mux.Handle("/v1/acl/token", s.wrapOpenTelemetry(s.ACLTokenSpecificRequest))
	s.mux.Handle("/v1/acl/token/", s.wrapOpenTelemetry(s.ACLTokenSpecificRequest))

	s.mux.Handle("/v1/client/fs/", wrapCORS(s.wrap(s.FsRequest)))
	s.mux.Handle("/v1/client/gc", s.wrapOpenTelemetry(s.ClientGCRequest))
	s.mux.Handle("/v1/client/stats", wrapCORS(s.wrap(s.ClientStatsRequest)))
	s.mux.Handle("/v1/client/allocation/", wrapCORS(s.wrap(s.ClientAllocRequest)))

	s.mux.Handle("/v1/agent/self", s.wrapOpenTelemetry(s.AgentSelfRequest))
	s.mux.Handle("/v1/agent/join", s.wrapOpenTelemetry(s.AgentJoinRequest))
	s.mux.Handle("/v1/agent/members", s.wrapOpenTelemetry(s.AgentMembersRequest))
	s.mux.Handle("/v1/agent/force-leave", s.wrapOpenTelemetry(s.AgentForceLeaveRequest))
	s.mux.Handle("/v1/agent/servers", s.wrapOpenTelemetry(s.AgentServersRequest))
	s.mux.Handle("/v1/agent/keyring/", s.wrapOpenTelemetry(s.KeyringOperationRequest))
	s.mux.Handle("/v1/agent/health", s.wrapOpenTelemetry(s.HealthRequest))
	s.mux.Handle("/v1/agent/host", s.wrapOpenTelemetry(s.AgentHostRequest))

	// Monitor is *not* an untrusted endpoint despite the log contents
	// potentially containing unsanitized user input. Monitor, like
	// "/v1/client/fs/logs", explicitly sets a "text/plain" or
	// "application/json" Content-Type depending on the ?plain= query
	// parameter.
	s.mux.Handle("/v1/agent/monitor", s.wrapOpenTelemetry(s.AgentMonitor))

	s.mux.HandleFunc("/v1/agent/pprof/", s.wrapNonJSON(s.AgentPprofRequest))

	s.mux.Handle("/v1/metrics", s.wrapOpenTelemetry(s.MetricsRequest))

	s.mux.Handle("/v1/validate/job", s.wrapOpenTelemetry(s.ValidateJobRequest))

	s.mux.Handle("/v1/regions", s.wrapOpenTelemetry(s.RegionListRequest))

	s.mux.Handle("/v1/scaling/policies", s.wrapOpenTelemetry(s.ScalingPoliciesRequest))
	s.mux.Handle("/v1/scaling/policy/", s.wrapOpenTelemetry(s.ScalingPolicySpecificRequest))

	s.mux.Handle("/v1/status/leader", s.wrapOpenTelemetry(s.StatusLeaderRequest))
	s.mux.Handle("/v1/status/peers", s.wrapOpenTelemetry(s.StatusPeersRequest))

	s.mux.Handle("/v1/search/fuzzy", s.wrapOpenTelemetry(s.FuzzySearchRequest))
	s.mux.Handle("/v1/search", s.wrapOpenTelemetry(s.SearchRequest))

	s.mux.Handle("/v1/operator/license", s.wrapOpenTelemetry(s.LicenseRequest))
	s.mux.Handle("/v1/operator/raft/", s.wrapOpenTelemetry(s.OperatorRequest))
	s.mux.Handle("/v1/operator/autopilot/configuration", s.wrapOpenTelemetry(s.OperatorAutopilotConfiguration))
	s.mux.Handle("/v1/operator/autopilot/health", s.wrapOpenTelemetry(s.OperatorServerHealth))
	s.mux.Handle("/v1/operator/snapshot", s.wrapOpenTelemetry(s.SnapshotRequest))

	s.mux.Handle("/v1/system/gc", s.wrapOpenTelemetry(s.GarbageCollectRequest))
	s.mux.Handle("/v1/system/reconcile/summaries", s.wrapOpenTelemetry(s.ReconcileJobSummaries))

	s.mux.Handle("/v1/operator/scheduler/configuration", s.wrapOpenTelemetry(s.OperatorSchedulerConfiguration))

	s.mux.Handle("/v1/event/stream", s.wrapOpenTelemetry(s.EventStream))
	s.mux.Handle("/v1/namespaces", s.wrapOpenTelemetry(s.NamespacesRequest))
	s.mux.Handle("/v1/namespace", s.wrapOpenTelemetry(s.NamespaceCreateRequest))
	s.mux.Handle("/v1/namespace/", s.wrapOpenTelemetry(s.NamespaceSpecificRequest))

	if uiEnabled {
		s.mux.Handle("/ui/", http.StripPrefix("/ui/", s.handleUI(http.FileServer(&UIAssetWrapper{FileSystem: assetFS()}))))
	} else {
		// Write the stubHTML
		s.mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(stubHTML))
		})
	}
	s.mux.Handle("/", s.handleRootFallthrough())

	if enableDebug {
		if !s.agent.config.DevMode {
			s.logger.Warn("enable_debug is set to true. This is insecure and should not be enabled in production")
		}
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

func (s *HTTPServer) handleUI(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		header := w.Header()
		header.Add("Content-Security-Policy", "default-src 'none'; connect-src *; img-src 'self' data:; script-src 'self'; style-src 'self' 'unsafe-inline'; form-action 'none'; frame-ancestors 'none'")
		h.ServeHTTP(w, req)
	})
}

func (s *HTTPServer) handleRootFallthrough() http.Handler {
	return s.auditHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			url := "/ui/"
			if req.URL.RawQuery != "" {
				url = url + "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, url, 307)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func errCodeFromHandler(err error) (int, string) {
	if err == nil {
		return 0, ""
	}

	code := 500
	errMsg := err.Error()
	if http, ok := err.(HTTPCodedError); ok {
		code = http.Code()
	} else if ecode, emsg, ok := structs.CodeFromRPCCodedErr(err); ok {
		code = ecode
		errMsg = emsg
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

	return code, errMsg
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPAPIResponseHeaders)
		// Invoke the handler
		reqURL := req.URL.String()
		start := time.Now()
		defer func() {
			s.logger.Debug("request complete", "method", req.Method, "path", reqURL, "duration", time.Since(start))
		}()
		obj, err := s.auditHandler(handler)(resp, req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			code := 500
			errMsg := err.Error()
			if http, ok := err.(HTTPCodedError); ok {
				code = http.Code()
			} else if ecode, emsg, ok := structs.CodeFromRPCCodedErr(err); ok {
				code = ecode
				errMsg = emsg
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
			if isAPIClientError(code) {
				s.logger.Debug("request failed", "method", req.Method, "path", reqURL, "error", err, "code", code)
			} else {
				s.logger.Error("request failed", "method", req.Method, "path", reqURL, "error", err, "code", code)
			}
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
				enc := codec.NewEncoder(&buf, structs.JsonHandleWithExtensions)
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

func (s *HTTPServer) wrapOpenTelemetry(f func(http.ResponseWriter, *http.Request) (interface{}, error)) http.Handler {
	fName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	parts := strings.Split(fName, ".")
	return otelhttp.NewHandler(http.HandlerFunc(s.wrap(f)), parts[3])
}

// wrapNonJSON is used to wrap functions returning non JSON
// serializeable data to make them more convenient. It is primarily
// responsible for setting nomad headers and logging.
// Handler functions are responsible for setting Content-Type Header
func (s *HTTPServer) wrapNonJSON(handler func(resp http.ResponseWriter, req *http.Request) ([]byte, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPAPIResponseHeaders)
		// Invoke the handler
		reqURL := req.URL.String()
		start := time.Now()
		defer func() {
			s.logger.Debug("request complete", "method", req.Method, "path", reqURL, "duration", time.Since(start))
		}()
		obj, err := s.auditNonJSONHandler(handler)(resp, req)

		// Check for an error
		if err != nil {
			code, errMsg := errCodeFromHandler(err)
			resp.WriteHeader(code)
			resp.Write([]byte(errMsg))
			if isAPIClientError(code) {
				s.logger.Debug("request failed", "method", req.Method, "path", reqURL, "error", err, "code", code)
			} else {
				s.logger.Error("request failed", "method", req.Method, "path", reqURL, "error", err, "code", code)
			}
			return
		}

		// write response
		if obj != nil {
			resp.Write(obj)
		}
	}
	return f
}

// isAPIClientError returns true if the passed http code represents a client error
func isAPIClientError(code int) bool {
	return 400 <= code && code <= 499
}

// decodeBody is used to decode a JSON request body
func decodeBody(req *http.Request, out interface{}) error {

	if req.Body == http.NoBody {
		return errors.New("Request body is empty")
	}

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
		resp.Header().Set(field, value)
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

// parseBool parses a query parameter to a boolean or returns (nil, nil) if the
// parameter is not present.
func parseBool(req *http.Request, field string) (*bool, error) {
	if str := req.URL.Query().Get(field); str != "" {
		param, err := strconv.ParseBool(str)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a bool: %v", field, str, err)
		}
		return &param, nil
	}

	return nil, nil
}

// parseToken is used to parse the X-Nomad-Token param
func (s *HTTPServer) parseToken(req *http.Request, token *string) {
	if other := req.Header.Get("X-Nomad-Token"); other != "" {
		*token = other
		return
	}
}

// parse is a convenience method for endpoints that need to parse multiple flags
// It sets r to the region and b to the QueryOptions in req
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, r *string, b *structs.QueryOptions) bool {
	s.parseRegion(req, r)
	s.parseToken(req, &b.AuthToken)
	parseConsistency(req, b)
	parsePrefix(req, b)
	parseNamespace(req, &b.Namespace)
	parsePagination(req, b)
	return parseWait(resp, req, b)
}

// parsePagination parses the pagination fields for QueryOptions
func parsePagination(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	rawPerPage := query.Get("per_page")
	if rawPerPage != "" {
		perPage, err := strconv.Atoi(rawPerPage)
		if err == nil {
			b.PerPage = int32(perPage)
		}
	}

	nextToken := query.Get("next_token")
	b.NextToken = nextToken
}

// parseWriteRequest is a convenience method for endpoints that need to parse a
// write request.
func (s *HTTPServer) parseWriteRequest(req *http.Request, w *structs.WriteRequest) {
	parseNamespace(req, &w.Namespace)
	s.parseToken(req, &w.AuthToken)
	s.parseRegion(req, &w.Region)
}

// wrapUntrustedContent wraps handlers in a http.ResponseWriter that prevents
// setting Content-Types that a browser may render (eg text/html). Any API that
// returns service-generated content (eg /v1/client/fs/cat) must be wrapped.
func (s *HTTPServer) wrapUntrustedContent(handler handlerFn) handlerFn {
	return func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		resp, closeWriter := noxssrw.NewResponseWriter(resp)
		defer func() {
			if _, err := closeWriter(); err != nil {
				// Can't write an error response at this point so just
				// log. s.wrap does not even log when resp.Write fails,
				// so log at low level.
				s.logger.Debug("error writing HTTP response", "error", err,
					"method", req.Method, "path", req.URL.String())
			}
		}()

		// Call the wrapped handler
		return handler(resp, req)
	}
}

// wrapCORS wraps a HandlerFunc in allowCORS and returns a http.Handler
func wrapCORS(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return allowCORS.Handler(http.HandlerFunc(f))
}
