// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

const (
	ctxKeyWebSocketConn      = "ws_connection"
	ctxKeyWebSocketAuthToken = "ws_auth_token"
)

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
func (s *HTTPServer) wrapWebsocketHandler(handler handlerFn) handlerFn {
	return func(w http.ResponseWriter, req *http.Request) (any, error) {

		// Upgrade the connection
		conn, err := s.wsUpgrader.Upgrade(w, req, nil)
		if err != nil {

			return "", fmt.Errorf("failed to upgrade connection: %v", err)
		}

		token, err := s.readWsHandshake(conn.ReadJSON, req)
		if err != nil {
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(400), err.Error()))
			return "", err
		}

		// Store connection and token in context for handler to use
		ctx := req.Context()
		ctx = context.WithValue(ctx, ctxKeyWebSocketConn, conn)
		ctx = context.WithValue(ctx, ctxKeyWebSocketAuthToken, token)
		*req = *req.WithContext(ctx)

		obj, err := handler(w, req)

		if err != nil {
			code, errMsg := errCodeFromHandler(err)
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(toWsCode(code), errMsg))
			return "", err
		}

		return obj, nil
	}
}

type wsHandshakeMessage struct {
	Version   int    `json:"version"`
	AuthToken string `json:"auth_token"`
}

// readWsHandshake reads the websocket handshake message and returns the auth token
func (s *HTTPServer) readWsHandshake(readFn func(interface{}) error, req *http.Request) (string, error) {
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
