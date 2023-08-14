// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/handlers"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-connlimit"
	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-msgpack/codec"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/rs/cors"
	"golang.org/x/time/rate"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/command/agent/event"
	"github.com/hashicorp/nomad/helper/noxssrw"
	"github.com/hashicorp/nomad/helper/tlsutil"
	"github.com/hashicorp/nomad/nomad"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

const (
	// ErrInvalidMethod is used if the HTTP method is not supported
	ErrInvalidMethod = "Invalid method"

	// ErrEntOnly is the error returned if accessing an enterprise only
	// endpoint
	ErrEntOnly = "Nomad Enterprise only endpoint"

	// ErrServerOnly is the error text returned if accessing a server only
	// endpoint
	ErrServerOnly = "Server only endpoint"

	// ContextKeyReqID is a unique ID for a given request
	ContextKeyReqID = "requestID"

	// MissingRequestID is a placeholder if we cannot retrieve a request
	// UUID from context
	MissingRequestID = "<missing request id>"
)

var (
	// Set to false by stub_asset if the ui build tag isn't enabled
	uiEnabled = true

	// Displayed when ui is disabled, but overridden if the ui build
	// tag isn't enabled
	stubHTML = "<html><p>Nomad UI is disabled</p></html>"

	// allowCORSWithMethods sets permissive CORS headers for a handler, used by
	// wrapCORS and wrapCORSWithMethods
	allowCORSWithMethods = func(methods ...string) *cors.Cors {
		return cors.New(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   methods,
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		})
	}
)

type handlerFn func(resp http.ResponseWriter, req *http.Request) (interface{}, error)
type handlerByteFn func(resp http.ResponseWriter, req *http.Request) ([]byte, error)

type RPCer interface {
	RPC(string, any, any) error
	Server() *nomad.Server
	Client() *client.Client
	Stats() map[string]map[string]string
	GetConfig() *Config
	GetMetricsSink() *metrics.InmemSink
}

// HTTPServer is used to wrap an Agent and expose it over an HTTP interface
type HTTPServer struct {
	agent RPCer

	// eventAuditor is the enterprise audit log feature which is needed by the
	// HTTP server.
	eventAuditor event.Auditor

	mux        *http.ServeMux
	listener   net.Listener
	listenerCh chan struct{}
	logger     log.Logger
	Addr       string

	wsUpgrader *websocket.Upgrader
}

// NewHTTPServers starts an HTTP server for every address.http configured in
// the agent.
func NewHTTPServers(agent *Agent, config *Config) ([]*HTTPServer, error) {
	var srvs []*HTTPServer
	var serverInitializationErrors error

	// Get connection handshake timeout limit
	handshakeTimeout, err := time.ParseDuration(config.Limits.HTTPSHandshakeTimeout)
	if err != nil {
		return srvs, fmt.Errorf("error parsing https_handshake_timeout: %v", err)
	} else if handshakeTimeout < 0 {
		return srvs, fmt.Errorf("https_handshake_timeout must be >= 0")
	}

	// Get max connection limit
	maxConns := 0
	if mc := config.Limits.HTTPMaxConnsPerClient; mc != nil {
		maxConns = *mc
	}
	if maxConns < 0 {
		return srvs, fmt.Errorf("http_max_conns_per_client must be >= 0")
	}

	tlsConf, err := tlsutil.NewTLSConfiguration(config.TLSConfig, config.TLSConfig.VerifyHTTPSClient, true)
	if err != nil && config.TLSConfig.EnableHTTP {
		return srvs, fmt.Errorf("failed to initialize HTTP server TLS configuration: %s", err)
	}

	wsUpgrader := &websocket.Upgrader{
		ReadBufferSize:  2048,
		WriteBufferSize: 2048,
	}

	// Start the listener
	for _, addr := range config.normalizedAddrs.HTTP {
		lnAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			serverInitializationErrors = multierror.Append(serverInitializationErrors, err)
			continue
		}
		ln, err := config.Listener("tcp", lnAddr.IP.String(), lnAddr.Port)
		if err != nil {
			serverInitializationErrors = multierror.Append(serverInitializationErrors, fmt.Errorf("failed to start HTTP listener: %v", err))
			continue
		}

		// If TLS is enabled, wrap the listener with a TLS listener
		if config.TLSConfig.EnableHTTP {
			tlsConfig, err := tlsConf.IncomingTLSConfig()
			if err != nil {
				serverInitializationErrors = multierror.Append(serverInitializationErrors, err)
				continue
			}
			ln = tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, tlsConfig)
		}

		// Create the server
		srv := &HTTPServer{
			agent:        agent,
			eventAuditor: agent.auditor,
			mux:          http.NewServeMux(),
			listener:     ln,
			listenerCh:   make(chan struct{}),
			logger:       agent.httpLogger,
			Addr:         ln.Addr().String(),
			wsUpgrader:   wsUpgrader,
		}
		srv.registerHandlers(config.EnableDebug)

		// Create HTTP server with timeouts
		httpServer := http.Server{
			Addr:      srv.Addr,
			Handler:   handlers.CompressHandler(srv.mux),
			ConnState: makeConnState(config.TLSConfig.EnableHTTP, handshakeTimeout, maxConns, srv.logger),
			ErrorLog:  newHTTPServerLogger(srv.logger),
		}

		go func() {
			defer close(srv.listenerCh)
			httpServer.Serve(ln)
		}()

		srvs = append(srvs, srv)
	}

	// Return early on errors
	if serverInitializationErrors != nil {
		for _, srv := range srvs {
			srv.Shutdown()
		}

		return srvs, serverInitializationErrors
	}

	// This HTTP server is only created when running in client mode, otherwise
	// the builtinDialer and builtinListener will be nil.
	if agent.builtinDialer != nil && agent.builtinListener != nil {
		srv := &HTTPServer{
			agent:        agent,
			eventAuditor: agent.auditor,
			mux:          http.NewServeMux(),
			listener:     agent.builtinListener,
			listenerCh:   make(chan struct{}),
			logger:       agent.httpLogger,
			Addr:         "builtin",
			wsUpgrader:   wsUpgrader,
		}

		srv.registerHandlers(config.EnableDebug)

		// builtinServer adds a wrapper to always authenticate requests
		httpServer := http.Server{
			Addr:     srv.Addr,
			Handler:  newAuthMiddleware(srv, srv.mux),
			ErrorLog: newHTTPServerLogger(srv.logger),
		}

		agent.taskAPIServer.SetServer(&httpServer)

		go func() {
			defer close(srv.listenerCh)
			httpServer.Serve(agent.builtinListener)
		}()

		// Don't append builtin servers to srvs as they don't need to be reloaded
		// when TLS changes. This does mean they need to be shutdown independently.
	}

	return srvs, nil
}

// makeConnState returns a ConnState func for use in an http.Server. If
// isTLS=true and handshakeTimeout>0 then the handshakeTimeout will be applied
// as a connection deadline to new connections and removed when the connection
// is active (meaning it has successfully completed the TLS handshake).
//
// If limit > 0, a per-address connection limit will be enabled regardless of
// TLS. If connLimit == 0 there is no connection limit.
func makeConnState(isTLS bool, handshakeTimeout time.Duration, connLimit int, logger log.Logger) func(conn net.Conn, state http.ConnState) {
	connLimiter := connLimiter(connLimit, logger)
	if !isTLS || handshakeTimeout == 0 {
		if connLimit > 0 {
			// Still return the connection limiter
			return connLimiter
		}
		return nil
	}

	if connLimit > 0 {
		// Return conn state callback with connection limiting and a
		// handshake timeout.

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

// connLimiter returns a connection-limiter function with a rate-limited 429-response error handler.
// The rate-limit prevents the TLS handshake necessary to write the HTTP response
// from consuming too many server resources.
func connLimiter(connLimit int, logger log.Logger) func(conn net.Conn, state http.ConnState) {
	// Global rate-limit of 10 responses per second with a 100-response burst.
	limiter := rate.NewLimiter(10, 100)

	tooManyConnsMsg := "Your IP is issuing too many concurrent connections, please rate limit your calls\n"
	tooManyRequestsResponse := []byte(fmt.Sprintf("HTTP/1.1 429 Too Many Requests\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n\r\n%s", len(tooManyConnsMsg), tooManyConnsMsg))
	return connlimit.NewLimiter(connlimit.Config{
		MaxConnsPerClientIP: connLimit,
	}).HTTPConnStateFuncWithErrorHandler(func(err error, conn net.Conn) {
		if err == connlimit.ErrPerClientIPLimitReached {
			metrics.IncrCounter([]string{"nomad", "agent", "http", "exceeded"}, 1)
			if n := limiter.Reserve(); n.Delay() == 0 {
				logger.Warn("Too many concurrent connections", "address", conn.RemoteAddr().String(), "limit", connLimit)
				conn.SetDeadline(time.Now().Add(10 * time.Millisecond))
				conn.Write(tooManyRequestsResponse)
			} else {
				n.Cancel()
			}
		}
		conn.Close()
	})
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

// ResolveToken extracts the ACL token secret ID from the request and
// translates it into an ACL object. Returns nil if ACLs are disabled.
func (s *HTTPServer) ResolveToken(req *http.Request) (*acl.ACL, error) {
	var secret string
	s.parseToken(req, &secret)

	var aclObj *acl.ACL
	var err error

	if srv := s.agent.Server(); srv != nil {
		aclObj, err = srv.ResolveToken(secret)
	} else {
		// Not a Server, so use the Client for token resolution. Note
		// this gets forwarded to a server with AllowStale = true if
		// the local ACL cache TTL has expired (30s by default)
		aclObj, err = s.agent.Client().ResolveToken(secret)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to resolve ACL token: %v", err)
	}

	return aclObj, nil
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	s.mux.HandleFunc("/v1/jobs", s.wrap(s.JobsRequest))
	s.mux.HandleFunc("/v1/jobs/parse", s.wrap(s.JobsParseRequest))
	s.mux.HandleFunc("/v1/job/", s.wrap(s.JobSpecificRequest))

	s.mux.HandleFunc("/v1/nodes", s.wrap(s.NodesRequest))
	s.mux.HandleFunc("/v1/node/", s.wrap(s.NodeSpecificRequest))

	s.mux.HandleFunc("/v1/node/pools", s.wrap(s.NodePoolsRequest))
	s.mux.HandleFunc("/v1/node/pool/", s.wrap(s.NodePoolSpecificRequest))

	s.mux.HandleFunc("/v1/allocations", s.wrap(s.AllocsRequest))
	s.mux.HandleFunc("/v1/allocation/", s.wrap(s.AllocSpecificRequest))

	s.mux.HandleFunc("/v1/evaluations", s.wrap(s.EvalsRequest))
	s.mux.HandleFunc("/v1/evaluations/count", s.wrap(s.EvalsCountRequest))
	s.mux.HandleFunc("/v1/evaluation/", s.wrap(s.EvalSpecificRequest))

	s.mux.HandleFunc("/v1/deployments", s.wrap(s.DeploymentsRequest))
	s.mux.HandleFunc("/v1/deployment/", s.wrap(s.DeploymentSpecificRequest))

	s.mux.HandleFunc("/v1/volumes", s.wrap(s.CSIVolumesRequest))
	s.mux.HandleFunc("/v1/volumes/external", s.wrap(s.CSIExternalVolumesRequest))
	s.mux.HandleFunc("/v1/volumes/snapshot", s.wrap(s.CSISnapshotsRequest))
	s.mux.HandleFunc("/v1/volume/csi/", s.wrap(s.CSIVolumeSpecificRequest))
	s.mux.HandleFunc("/v1/plugins", s.wrap(s.CSIPluginsRequest))
	s.mux.HandleFunc("/v1/plugin/csi/", s.wrap(s.CSIPluginSpecificRequest))

	s.mux.HandleFunc("/v1/acl/policies", s.wrap(s.ACLPoliciesRequest))
	s.mux.HandleFunc("/v1/acl/policy/", s.wrap(s.ACLPolicySpecificRequest))

	s.mux.HandleFunc("/v1/acl/token/onetime", s.wrap(s.UpsertOneTimeToken))
	s.mux.HandleFunc("/v1/acl/token/onetime/exchange", s.wrap(s.ExchangeOneTimeToken))
	s.mux.HandleFunc("/v1/acl/bootstrap", s.wrap(s.ACLTokenBootstrap))
	s.mux.HandleFunc("/v1/acl/tokens", s.wrap(s.ACLTokensRequest))
	s.mux.HandleFunc("/v1/acl/token", s.wrap(s.ACLTokenSpecificRequest))
	s.mux.HandleFunc("/v1/acl/token/", s.wrap(s.ACLTokenSpecificRequest))

	// Register our ACL role handlers.
	s.mux.HandleFunc("/v1/acl/roles", s.wrap(s.ACLRoleListRequest))
	s.mux.HandleFunc("/v1/acl/role", s.wrap(s.ACLRoleRequest))
	s.mux.HandleFunc("/v1/acl/role/", s.wrap(s.ACLRoleSpecificRequest))

	// Register our ACL auth-method handlers.
	s.mux.HandleFunc("/v1/acl/auth-methods", s.wrap(s.ACLAuthMethodListRequest))
	s.mux.HandleFunc("/v1/acl/auth-method", s.wrap(s.ACLAuthMethodRequest))
	s.mux.HandleFunc("/v1/acl/auth-method/", s.wrap(s.ACLAuthMethodSpecificRequest))

	// Register our ACL binding rule handlers.
	s.mux.HandleFunc("/v1/acl/binding-rules", s.wrap(s.ACLBindingRuleListRequest))
	s.mux.HandleFunc("/v1/acl/binding-rule", s.wrap(s.ACLBindingRuleRequest))
	s.mux.HandleFunc("/v1/acl/binding-rule/", s.wrap(s.ACLBindingRuleSpecificRequest))

	// Register out ACL OIDC SSO and auth handlers.
	s.mux.HandleFunc("/v1/acl/oidc/auth-url", s.wrap(s.ACLOIDCAuthURLRequest))
	s.mux.HandleFunc("/v1/acl/oidc/complete-auth", s.wrap(s.ACLOIDCCompleteAuthRequest))
	s.mux.HandleFunc("/v1/acl/login", s.wrap(s.ACLLoginRequest))

	s.mux.Handle("/v1/client/fs/", wrapCORS(s.wrap(s.FsRequest)))
	s.mux.HandleFunc("/v1/client/gc", s.wrap(s.ClientGCRequest))
	s.mux.Handle("/v1/client/stats", wrapCORS(s.wrap(s.ClientStatsRequest)))
	s.mux.Handle("/v1/client/allocation/", wrapCORS(s.wrap(s.ClientAllocRequest)))
	s.mux.Handle("/v1/client/metadata", wrapCORS(s.wrap(s.NodeMetaRequest)))

	s.mux.HandleFunc("/v1/agent/self", s.wrap(s.AgentSelfRequest))
	s.mux.HandleFunc("/v1/agent/join", s.wrap(s.AgentJoinRequest))
	s.mux.HandleFunc("/v1/agent/members", s.wrap(s.AgentMembersRequest))
	s.mux.HandleFunc("/v1/agent/force-leave", s.wrap(s.AgentForceLeaveRequest))
	s.mux.HandleFunc("/v1/agent/servers", s.wrap(s.AgentServersRequest))
	s.mux.HandleFunc("/v1/agent/schedulers", s.wrap(s.AgentSchedulerWorkerInfoRequest))
	s.mux.HandleFunc("/v1/agent/schedulers/config", s.wrap(s.AgentSchedulerWorkerConfigRequest))
	s.mux.HandleFunc("/v1/agent/keyring/", s.wrap(s.KeyringOperationRequest))
	s.mux.HandleFunc("/v1/agent/health", s.wrap(s.HealthRequest))
	s.mux.HandleFunc("/v1/agent/host", s.wrap(s.AgentHostRequest))

	// Register our service registration handlers.
	s.mux.HandleFunc("/v1/services", s.wrap(s.ServiceRegistrationListRequest))
	s.mux.HandleFunc("/v1/service/", s.wrap(s.ServiceRegistrationRequest))

	// Monitor is *not* an untrusted endpoint despite the log contents
	// potentially containing unsanitized user input. Monitor, like
	// "/v1/client/fs/logs", explicitly sets a "text/plain" or
	// "application/json" Content-Type depending on the ?plain= query
	// parameter.
	s.mux.HandleFunc("/v1/agent/monitor", s.wrap(s.AgentMonitor))

	s.mux.HandleFunc("/v1/agent/pprof/", s.wrapNonJSON(s.AgentPprofRequest))

	s.mux.HandleFunc("/v1/metrics", s.wrap(s.MetricsRequest))

	s.mux.HandleFunc("/v1/validate/job", s.wrap(s.ValidateJobRequest))

	s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))

	s.mux.HandleFunc("/v1/scaling/policies", s.wrap(s.ScalingPoliciesRequest))
	s.mux.HandleFunc("/v1/scaling/policy/", s.wrap(s.ScalingPolicySpecificRequest))

	s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeaderRequest))
	s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeersRequest))

	s.mux.HandleFunc("/v1/search/fuzzy", s.wrap(s.FuzzySearchRequest))
	s.mux.HandleFunc("/v1/search", s.wrap(s.SearchRequest))
	s.mux.HandleFunc("/v1/operator/license", s.wrap(s.LicenseRequest))
	s.mux.HandleFunc("/v1/operator/raft/", s.wrap(s.OperatorRequest))
	s.mux.HandleFunc("/v1/operator/keyring/", s.wrap(s.KeyringRequest))
	s.mux.HandleFunc("/v1/operator/autopilot/configuration", s.wrap(s.OperatorAutopilotConfiguration))
	s.mux.HandleFunc("/v1/operator/autopilot/health", s.wrap(s.OperatorServerHealth))
	s.mux.HandleFunc("/v1/operator/snapshot", s.wrap(s.SnapshotRequest))

	s.mux.HandleFunc("/v1/system/gc", s.wrap(s.GarbageCollectRequest))
	s.mux.HandleFunc("/v1/system/reconcile/summaries", s.wrap(s.ReconcileJobSummaries))

	s.mux.HandleFunc("/v1/operator/scheduler/configuration", s.wrap(s.OperatorSchedulerConfiguration))

	s.mux.HandleFunc("/v1/event/stream", s.wrap(s.EventStream))

	s.mux.HandleFunc("/v1/namespaces", s.wrap(s.NamespacesRequest))
	s.mux.HandleFunc("/v1/namespace", s.wrap(s.NamespaceCreateRequest))
	s.mux.HandleFunc("/v1/namespace/", s.wrap(s.NamespaceSpecificRequest))

	s.mux.Handle("/v1/vars", wrapCORS(s.wrap(s.VariablesListRequest)))
	s.mux.Handle("/v1/var/", wrapCORSWithAllowedMethods(s.wrap(s.VariableSpecificRequest), "HEAD", "GET", "PUT", "DELETE"))

	// JWKS Handler
	s.mux.HandleFunc("/.well-known/jwks.json", s.wrap(s.JWKSRequest))

	agentConfig := s.agent.GetConfig()
	uiConfigEnabled := agentConfig.UI != nil && agentConfig.UI.Enabled

	if uiEnabled && uiConfigEnabled {
		s.mux.Handle("/ui/", http.StripPrefix("/ui/", s.handleUI(agentConfig.UI.ContentSecurityPolicy, http.FileServer(&UIAssetWrapper{FileSystem: assetFS()}))))
		s.logger.Debug("UI is enabled")
	} else {
		// Write the stubHTML
		s.mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(stubHTML))
		})
		if uiEnabled && !uiConfigEnabled {
			s.logger.Warn("UI is disabled")
		} else {
			s.logger.Debug("UI is disabled in this build")
		}
	}
	s.mux.Handle("/", s.handleRootFallthrough())

	if enableDebug {
		if !agentConfig.DevMode {
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

// builtinAPI is a wrapper around serving the HTTP API to arbitrary listeners
// such as the Task API. It is necessary because the HTTP servers are created
// *after* the client has been initialized, so this wrapper blocks Serve
// requests from task api hooks until the HTTP server is setup and ready to
// accept from new listeners.
//
// bufconndialer provides similar functionality to consul-template except it
// satisfies the Dialer API as opposed to the Serve(Listener) API.
type builtinAPI struct {
	// srvReadyCh is closed when srv is ready
	srvReadyCh chan struct{}

	// srv is a builtin http server. Must lock around setting as it could happen
	// concurrently with shutting down.
	srv     *http.Server
	srvLock sync.Mutex
}

func newBuiltinAPI() *builtinAPI {
	return &builtinAPI{
		srvReadyCh: make(chan struct{}),
	}
}

// SetServer sets the API HTTP server for Serve to add listeners to.
//
// It must be called exactly once and will noop on subsequent calls.
func (b *builtinAPI) SetServer(srv *http.Server) {
	select {
	case <-b.srvReadyCh:
		return
	default:
	}

	b.srvLock.Lock()
	defer b.srvLock.Unlock()

	b.srv = srv
	close(b.srvReadyCh)
}

// Serve the HTTP API on the listener unless the context is canceled before the
// HTTP API is ready to serve listeners. A non-nil error will always be
// returned, but http.ErrServerClosed and net.ErrClosed can likely be ignored
// as they indicate the server or listener is being shutdown.
func (b *builtinAPI) Serve(ctx context.Context, l net.Listener) error {
	select {
	case <-ctx.Done():
		// Caller canceled context before server was ready.
		return ctx.Err()
	case <-b.srvReadyCh:
		// Server ready for listeners! Continue on...
	}

	return b.srv.Serve(l)
}

func (b *builtinAPI) Shutdown() {
	b.srvLock.Lock()
	defer b.srvLock.Unlock()

	if b.srv != nil {
		b.srv.Close()
	}

	select {
	case <-b.srvReadyCh:
	default:
		close(b.srvReadyCh)
	}
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

func (s *HTTPServer) handleUI(policy *config.ContentSecurityPolicy, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		header := w.Header()
		header.Add("Content-Security-Policy", policy.String())
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
			http.Redirect(w, req, url, http.StatusTemporaryRedirect)
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
		} else if strings.HasSuffix(errMsg, structs.ErrJobRegistrationDisabled.Error()) {
			errMsg = structs.ErrJobRegistrationDisabled.Error()
			code = 403
		}
	}

	return code, errMsg
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.GetConfig().HTTPAPIResponseHeaders)
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
				} else if strings.HasSuffix(errMsg, structs.ErrJobRegistrationDisabled.Error()) {
					errMsg = structs.ErrJobRegistrationDisabled.Error()
					code = 403
				} else if strings.HasSuffix(errMsg, structs.ErrIncompatibleFiltering.Error()) {
					errMsg = structs.ErrIncompatibleFiltering.Error()
					code = 400
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

// wrapNonJSON is used to wrap functions returning non JSON
// serializeable data to make them more convenient. It is primarily
// responsible for setting nomad headers and logging.
// Handler functions are responsible for setting Content-Type Header
func (s *HTTPServer) wrapNonJSON(handler func(resp http.ResponseWriter, req *http.Request) ([]byte, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.GetConfig().HTTPAPIResponseHeaders)
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

// setNextToken is used to set the next token header for pagination
func setNextToken(resp http.ResponseWriter, nextToken string) {
	if nextToken != "" {
		resp.Header().Set("X-Nomad-NextToken", nextToken)
	}
}

// setMeta is used to set the query response meta data
func setMeta(resp http.ResponseWriter, m *structs.QueryMeta) {
	setIndex(resp, m.Index)
	setLastContact(resp, m.LastContact)
	setKnownLeader(resp, m.KnownLeader)
	setNextToken(resp, m.NextToken)
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
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte("Invalid wait time"))
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte("Invalid index"))
			return true
		}
		b.MinQueryIndex = index
	}
	return false
}

// parseConsistency is used to parse the ?stale query params.
func parseConsistency(resp http.ResponseWriter, req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	if staleVal, ok := query["stale"]; ok {
		if len(staleVal) == 0 || staleVal[0] == "" {
			b.AllowStale = true
			return
		}
		staleQuery, err := strconv.ParseBool(staleVal[0])
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			_, _ = resp.Write([]byte(fmt.Sprintf("Expect `true` or `false` for `stale` query string parameter, got %s", staleVal[0])))
			return
		}
		b.AllowStale = staleQuery
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
		*r = s.agent.GetConfig().Region
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

// parseIdempotencyToken is used to parse the ?idempotency_token parameter
func parseIdempotencyToken(req *http.Request, n *string) {
	if idempotencyToken := req.URL.Query().Get("idempotency_token"); idempotencyToken != "" {
		*n = idempotencyToken
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

// parseInt parses a query parameter to a int or returns (nil, nil) if the
// parameter is not present.
func parseInt(req *http.Request, field string) (*int, error) {
	if str := req.URL.Query().Get(field); str != "" {
		param, err := strconv.Atoi(str)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse value of %q (%v) as a int: %v", field, str, err)
		}
		return &param, nil
	}
	return nil, nil
}

// parseToken is used to parse the X-Nomad-Token param
func (s *HTTPServer) parseToken(req *http.Request, token *string) {
	if other := req.Header.Get("X-Nomad-Token"); other != "" {
		*token = strings.TrimSpace(other)
		return
	}

	if other := req.Header.Get("Authorization"); other != "" {
		// HTTP Authorization headers are in the format: <Scheme>[SPACE]<Value>
		// Ref. https://tools.ietf.org/html/rfc7236#section-3
		parts := strings.Split(other, " ")

		// Authorization Header is invalid if containing 1 or 0 parts, e.g.:
		// "" || "<Scheme><Value>" || "<Scheme>" || "<Value>"
		if len(parts) > 1 {
			scheme := parts[0]
			// Everything after "<Scheme>" is "<Value>", trimmed
			value := strings.TrimSpace(strings.Join(parts[1:], " "))

			// <Scheme> must be "Bearer"
			if strings.ToLower(scheme) == "bearer" {
				// Since Bearer tokens shouldn't contain spaces (rfc6750#section-2.1)
				// "value" is tokenized, only the first item is used
				*token = strings.TrimSpace(strings.Split(value, " ")[0])
			}
		}
	}
}

// parse is a convenience method for endpoints that need to parse multiple flags
// It sets r to the region and b to the QueryOptions in req
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, r *string, b *structs.QueryOptions) bool {
	s.parseRegion(req, r)
	s.parseToken(req, &b.AuthToken)
	parseConsistency(resp, req, b)
	parsePrefix(req, b)
	parseNamespace(req, &b.Namespace)
	parsePagination(req, b)
	parseFilter(req, b)
	parseReverse(req, b)
	return parseWait(resp, req, b)
}

// parsePagination parses the pagination fields for QueryOptions
func parsePagination(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	rawPerPage := query.Get("per_page")
	if rawPerPage != "" {
		perPage, err := strconv.ParseInt(rawPerPage, 10, 32)
		if err == nil {
			b.PerPage = int32(perPage)
		}
	}

	b.NextToken = query.Get("next_token")
}

// parseFilter parses the filter query parameter for QueryOptions
func parseFilter(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	if filter := query.Get("filter"); filter != "" {
		b.Filter = filter
	}
}

// parseReverse parses the reverse query parameter for QueryOptions
func parseReverse(req *http.Request, b *structs.QueryOptions) {
	query := req.URL.Query()
	b.Reverse = query.Get("reverse") == "true"
}

// parseNode parses the node_id query parameter for node specific requests.
func parseNode(req *http.Request, nodeID *string) {
	if n := req.URL.Query().Get("node_id"); n != "" {
		*nodeID = n
	}
}

// parseNodeListStubFields parses query parameters related to node list stubs
// fields.
func parseNodeListStubFields(req *http.Request) (*structs.NodeStubFields, error) {
	fields := &structs.NodeStubFields{}

	// Parse resources field selection.
	resources, err := parseBool(req, "resources")
	if err != nil {
		return nil, err
	}
	if resources != nil {
		fields.Resources = *resources
	}

	// Parse OS field selection.
	os, err := parseBool(req, "os")
	if err != nil {
		return nil, err
	}
	if os != nil {
		fields.OS = *os
	}

	return fields, nil
}

// parseWriteRequest is a convenience method for endpoints that need to parse a
// write request.
func (s *HTTPServer) parseWriteRequest(req *http.Request, w *structs.WriteRequest) {
	parseNamespace(req, &w.Namespace)
	s.parseToken(req, &w.AuthToken)
	s.parseRegion(req, &w.Region)
	parseIdempotencyToken(req, &w.IdempotencyToken)
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

// wrapCORS wraps a HandlerFunc in allowCORS with read ("HEAD", "GET") methods
// and returns a http.Handler
func wrapCORS(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return wrapCORSWithAllowedMethods(f, "HEAD", "GET")
}

// wrapCORSWithAllowedMethods wraps a HandlerFunc in an allowCORS with the given
// method list and returns a http.Handler
func wrapCORSWithAllowedMethods(f func(http.ResponseWriter, *http.Request), methods ...string) http.Handler {
	return allowCORSWithMethods(methods...).Handler(http.HandlerFunc(f))
}

// authMiddleware implements the http.Handler interface to enforce
// authentication for *all* requests. Even with ACLs enabled there are
// endpoints which are accessible without authenticating. This middleware is
// used for the Task API to enfoce authentication for all API access.
type authMiddleware struct {
	srv     *HTTPServer
	wrapped http.Handler
}

func newAuthMiddleware(srv *HTTPServer, h http.Handler) http.Handler {
	return &authMiddleware{
		srv:     srv,
		wrapped: h,
	}
}

func (a *authMiddleware) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	args := structs.GenericRequest{}
	reply := structs.ACLWhoAmIResponse{}
	if a.srv.parse(resp, req, &args.Region, &args.QueryOptions) {
		// Error parsing request, 400
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte(http.StatusText(http.StatusBadRequest)))
		return
	}

	if args.AuthToken == "" {
		// 401 instead of 403 since no token was present.
		resp.WriteHeader(http.StatusUnauthorized)
		resp.Write([]byte(http.StatusText(http.StatusUnauthorized)))
		return
	}

	if err := a.srv.agent.RPC("ACL.WhoAmI", &args, &reply); err != nil {
		// When ACLs are enabled, WhoAmI returns ErrPermissionDenied on bad
		// credentials, so convert it to a Forbidden response code.
		if strings.HasSuffix(err.Error(), structs.ErrPermissionDenied.Error()) {
			a.srv.logger.Debug("Failed to authenticated Task API request", "method", req.Method, "url", req.URL)
			resp.WriteHeader(http.StatusForbidden)
			resp.Write([]byte(http.StatusText(http.StatusForbidden)))
			return
		}

		a.srv.logger.Error("error authenticating built API request", "error", err, "url", req.URL, "method", req.Method)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte("Server error authenticating request\n"))
		return
	}

	// Require an acl token or workload identity
	if reply.Identity == nil || (reply.Identity.ACLToken == nil && reply.Identity.Claims == nil) {
		a.srv.logger.Debug("Failed to authenticated Task API request", "method", req.Method, "url", req.URL)
		resp.WriteHeader(http.StatusForbidden)
		resp.Write([]byte(http.StatusText(http.StatusForbidden)))
		return
	}

	a.srv.logger.Trace("Authenticated request", "id", reply.Identity, "method", req.Method, "url", req.URL)
	a.wrapped.ServeHTTP(resp, req)
}
