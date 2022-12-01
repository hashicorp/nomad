package oidc

import (
	"fmt"
	"net"
	"net/http"

	"github.com/hashicorp/cap/oidc"
	"github.com/hashicorp/nomad/api"
)

// CallbackServer is started with NewCallbackServer and creates an HTTP
// server for handling loopback OIDC auth redirects.
type CallbackServer struct {
	ln          net.Listener
	url         string
	clientNonce string
	errCh       chan error
	successCh   chan *api.ACLOIDCCompleteAuthRequest
}

// NewCallbackServer creates and starts a new local HTTP server for
// OIDC authentication to redirect to. This is used to capture the
// necessary information to complete the authentication.
func NewCallbackServer(addr string) (*CallbackServer, error) {
	// Generate our nonce
	nonce, err := oidc.NewID()
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	// Initialize our callback server
	srv := &CallbackServer{
		url:         fmt.Sprintf("http://%s/oidc/callback", ln.Addr().String()),
		ln:          ln,
		clientNonce: nonce,
		errCh:       make(chan error, 5),
		successCh:   make(chan *api.ACLOIDCCompleteAuthRequest, 5),
	}

	// Register our HTTP route and start the server
	mux := http.NewServeMux()
	mux.Handle("/oidc/callback", srv)
	go func() {
		httpServer := &http.Server{Handler: mux}
		if err := httpServer.Serve(ln); err != nil {
			srv.errCh <- err
		}
	}()

	return srv, nil
}

// Close cleans up and shuts down the server. On close, errors may be
// sent to ErrorCh and should be ignored.
func (s *CallbackServer) Close() error { return s.ln.Close() }

// RedirectURI is the redirect URI that should be provided for the auth.
func (s *CallbackServer) RedirectURI() string { return s.url }

// Nonce returns a generated nonce that can be used for the request.
func (s *CallbackServer) Nonce() string { return s.clientNonce }

// ErrorCh returns a channel where any errors are sent. Errors may be
// sent after Close and should be disregarded.
func (s *CallbackServer) ErrorCh() <-chan error { return s.errCh }

// SuccessCh returns a channel that gets sent a partially completed
// request to complete the OIDC auth with the Waypoint server.
func (s *CallbackServer) SuccessCh() <-chan *api.ACLOIDCCompleteAuthRequest { return s.successCh }

// ServeHTTP implements http.Handler and handles the callback request. This
// isn't usually used directly; use the server address instead.
func (s *CallbackServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()

	// Build our result
	result := &api.ACLOIDCCompleteAuthRequest{
		State:       q.Get("state"),
		ClientNonce: s.clientNonce,
		Code:        q.Get("code"),
	}

	// Send our result. We don't block here because the channel should be
	// buffered, and otherwise we're done.
	select {
	case s.successCh <- result:
	default:
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Authentication complete. You may now close this window and return to your terminal."))
}
