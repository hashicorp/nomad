// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"crypto/fips140"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

const (
	ctxKeyWebSocketConn      = "ws_connection"
	ctxKeyWebSocketAuthToken = "ws_auth_token"
)

const (
	// The watcher protocol assumes that no reads will be performed
	// on the websocket.
	websocketProtocolWatcher = "nomad-watcher"
)

// wsResponse is the default websocket response format.
type wsResponse struct {
	Headers http.Header `json:"headers"`
	Payload any         `json:"payload"`
}

// isWebsocketUpgrade checks if the request is a websocket upgrade request.
func isWebsocketUpgrade(req *http.Request) bool {
	return websocket.IsWebSocketUpgrade(req)
}

// wrapWebsocketHandler upgrades the HTTP connection to a websocket. Auditing
// for websockets gets complicated because browsers won't send the X-Nomad-Token
// header and only authenticate via the first message. This means we have to
// upgrade the connection, write audit logs, and then hand off the
// already-upgraded connection to the handler. We pass the connection and the
// auth token via request context.
//
// NOTE: Outside of initial setup/upgrade failures, this handler should not
// return any value or error after the connection has been upgraded to a
// websocket. Any value or error should be sent to the websocket connection
// along with a close message before returning.
func (s *HTTPServer) wrapWebsocketHandler(handler handlerFn) handlerFn {
	return func(w http.ResponseWriter, req *http.Request) (any, error) {

		if fips140.Enabled() {
			return "", fmt.Errorf("websockets are disallowed in FIPS-140 mode")
		}

		// Upgrade the connection
		conn, err := s.wsUpgrader.Upgrade(w, req, nil)
		if err != nil {
			// The upgrade failed so return the error.
			return nil, fmt.Errorf("failed to upgrade connection: %v", err)
		}

		// Ensure the underlying websocket connection is closed when we
		// are done. This does not transmit the close message to the
		// client, so that must still be done before returning from
		// this function. Failure to send the close message can cause
		// the client connection to hang for an extended period of time
		// before closing.
		defer conn.Close()

		token, err := s.readWsHandshake(conn.ReadJSON, req)
		if err != nil {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(400), err.Error()))
			return nil, nil
		}

		// Store connection and token in context for handler to use
		ctx := req.Context()
		ctx = context.WithValue(ctx, ctxKeyWebSocketConn, conn)
		ctx = context.WithValue(ctx, ctxKeyWebSocketAuthToken, token)
		*req = *req.WithContext(ctx)

		// Check the websocket subprotocol and perform any required setup
		switch conn.Subprotocol() {
		case websocketProtocolWatcher:
			go s.runWebsocketWatcher(conn)
		}

		// Run the handler to process the request.
		obj, err := handler(w, req)

		// If an error was encountered, send the close control message
		// with error information.
		if err != nil {
			code, errMsg := errCodeFromHandler(err)
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(code), errMsg))

			return nil, nil
		}

		// If the websocket is not configured with a subprotocol and
		// the handler result is empty, do not write anything to the
		// websocket. This retains pre-existing behavior.
		if obj == nil && conn.Subprotocol() == "" {
			return nil, nil
		}

		// Construct the result to return.
		result := wsResponse{
			Headers: w.Header(),
			Payload: obj,
		}

		// Write the result to the websocket.
		if err := conn.WriteJSON(result); err != nil {
			s.logger.Error("failed to write result to websocket", "error", err)
			// Attempt to send a close message with error information.
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "failed to write response"))

			return nil, nil
		}

		// Signal to the client that the connection is closing.
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "request complete"))

		return nil, nil
	}
}

type wsHandshakeMessage struct {
	Version   int    `json:"version"`
	AuthToken string `json:"auth_token"`
}

// readWsHandshake reads the websocket handshake message and returns the auth token
func (s *HTTPServer) readWsHandshake(readFn func(any) error, req *http.Request) (string, error) {
	// Avoid handshake if request doesn't require one
	if hv := req.URL.Query().Get("ws_handshake"); hv == "" {
		return "", nil
	} else if h, err := strconv.ParseBool(hv); err != nil {
		return "", fmt.Errorf("ws_handshake value is not a boolean: %v", err)
	} else if !h {
		return "", nil
	}

	// verify that any header token set by a non-browser client agrees with the
	// auth header
	reqToken := new(string)
	s.parseToken(req, reqToken)

	var h wsHandshakeMessage
	err := readFn(&h)
	if err != nil {
		return "", err
	}

	if reqToken != nil && *reqToken != "" && *reqToken != h.AuthToken {
		return "", fmt.Errorf("handshake auth token mismatched auth header token")
	}

	supportedWSHandshakeVersion := 1
	if h.Version != supportedWSHandshakeVersion {
		return "", fmt.Errorf("unexpected handshake value: %v", h.Version)
	}

	return h.AuthToken, nil
}

// getWebsocketConnection retrieves the websocket connection from context
func (s *HTTPServer) getWebsocketConnection(req *http.Request) (*websocket.Conn, error) {
	ctx := req.Context()

	// Get websocket connection from context (set by audit wrapper)
	connRaw := ctx.Value(ctxKeyWebSocketConn)
	if connRaw == nil {
		return nil, fmt.Errorf("websocket connection not found in context")
	}
	conn, ok := connRaw.(*websocket.Conn)
	if !ok {
		return nil, fmt.Errorf("invalid websocket connection in context")
	}

	return conn, nil
}

// runWebsocketWatcher reads messages from a websocket using the watcher
// protocol. That usage does not read from the websocket, but reads are
// required to receive control messages so this will continually read
// the websocket until an error is encountered.
func (s *HTTPServer) runWebsocketWatcher(conn *websocket.Conn) {
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.logger.Trace("watcher websocket reader encountered error, stopping read", "error", err)
			return
		}
	}
}
