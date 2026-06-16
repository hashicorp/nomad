// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/nomad/v2/ci"
	cstructs "github.com/hashicorp/nomad/v2/client/structs"
	"github.com/hashicorp/nomad/v2/helper/uuid"
	"github.com/hashicorp/nomad/v2/nomad/structs"
	"github.com/shoenig/test/must"
)

// TestHTTP_WrapWebsocketHandler exercises the the wrapper for the websockets,
// making sure that the inner handler gets the auth token
func TestHTTP_WrapWebsocketHandler(t *testing.T) {
	ci.Parallel(t)

	httpACLTest(t, nil, func(ta *TestAgent) {
		s := ta.Server

		rpcHandler := mockStreamingRpcHandler(t, [][]byte{
			[]byte("one"), []byte("two"), []byte("done!")})

		wsHandler := func(resp http.ResponseWriter, req *http.Request) (any, error) {
			args := cstructs.AllocExecRequest{
				AllocID: uuid.Generate(),
				Task:    "foo",
				Cmd:     []string{"bar"},
			}
			s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

			conn, err := s.getWebsocketConnection(req)
			if err != nil {
				return nil, err
			}
			if args.QueryOptions.AuthToken != ta.RootToken.SecretID {
				return nil, structs.ErrPermissionDenied
			}

			_, err = s.execStreamImpl(conn, &args, rpcHandler)
			return nil, err
		}

		srv := httptest.NewServer(http.HandlerFunc(s.wrap(wsHandler)))
		t.Cleanup(srv.Close)

		cases := []struct {
			name        string
			msgToken    string
			headerToken string
			expectMsg   []string
			expectErr   string
		}{
			{
				name:      "token from browser",
				msgToken:  ta.RootToken.SecretID,
				expectMsg: []string{"one", "two", "done!"},
			},
			{
				name:        "token from CLI",
				headerToken: ta.RootToken.SecretID,
				expectMsg:   []string{"one", "two", "done!"},
			},
			{
				name:      "unauthenticated",
				expectErr: "Permission denied",
				expectMsg: []string{"websocket: close 1008 (policy violation): Permission denied"},
			},
			{
				name:        "attempted mismatch token",
				headerToken: ta.RootToken.SecretID,
				msgToken:    uuid.Generate(),
				expectMsg:   []string{"websocket: close 1008 (policy violation): handshake auth token mismatched auth header token"},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {

				url := strings.Replace(srv.URL, "http://", "ws://", 1)
				var conn *websocket.Conn
				var err error

				switch {
				case tc.headerToken != "" && tc.msgToken != "":
					url += "?ws_handshake=1"
					h := http.Header{}
					h.Set("X-Nomad-Token", tc.headerToken)
					conn, _, err = websocket.DefaultDialer.Dial(url, h)
					must.NoError(t, err, must.Sprint("failed to dial"))
					handshake := wsHandshakeMessage{Version: 1, AuthToken: tc.msgToken}
					must.NoError(t, conn.WriteJSON(handshake))

				case tc.headerToken != "":
					h := http.Header{}
					h.Set("X-Nomad-Token", tc.headerToken)
					conn, _, err = websocket.DefaultDialer.Dial(url, h)
					must.NoError(t, err, must.Sprint("failed to dial"))

				case tc.msgToken != "":
					url += "?ws_handshake=1"
					conn, _, err = websocket.DefaultDialer.Dial(url, nil)
					must.NoError(t, err, must.Sprint("failed to dial"))
					handshake := wsHandshakeMessage{Version: 1, AuthToken: tc.msgToken}
					must.NoError(t, conn.WriteJSON(handshake))

				default:
					conn, _, err = websocket.DefaultDialer.Dial(url, nil)
					must.NoError(t, err, must.Sprint("failed to dial"))
				}

				t.Cleanup(func() { _ = conn.Close() })

				drainConn := func() []string {
					resp := []string{}

					for {
						select {
						case <-t.Context().Done():
							return resp
						default:
							_, message, err := conn.ReadMessage()
							if err != nil {
								if !isClosedError(err) {
									resp = append(resp, err.Error())
									return resp
								}
								return resp
							}
							resp = append(resp, string(message))
						}
					}
				}
				resp := drainConn()
				must.SliceContainsAll(t, tc.expectMsg, resp, must.Sprintf("%+v", resp))
			})
		}

	})

}

// TestHTTP_ReadWsHandshake exercises the extracting the auth token from the
// websocket handshake
func TestHTTP_ReadWsHandshake(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name        string
		token       string
		handshake   bool
		headerToken string
		expectErr   string
	}{
		{
			name:      "plain compatible mode",
			token:     "",
			handshake: false,
		},
		{
			name:      "handshake unauthenticated",
			token:     "",
			handshake: true,
		},
		{
			name:      "handshake authenticated",
			token:     "mysupersecret",
			handshake: true,
		},
		{
			name:        "handshake mismatch",
			token:       "mysupersecret",
			handshake:   true,
			headerToken: "evil",
			expectErr:   "handshake auth token mismatched auth header token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {

			called := false
			readFn := func(h interface{}) error {
				called = true
				if !tc.handshake {
					return fmt.Errorf("should not be called")
				}

				hm := h.(*wsHandshakeMessage)
				hm.Version = 1
				hm.AuthToken = tc.token
				return nil
			}

			req := httptest.NewRequest(http.MethodPut, "/target", nil)
			if tc.handshake {
				req.URL.RawQuery = "ws_handshake=true"
			}
			if tc.headerToken != "" {
				req.Header.Add("X-Nomad-Token", tc.headerToken)
			}

			s := &HTTPServer{}
			authToken, err := s.readWsHandshake(readFn, req)
			if tc.expectErr != "" {
				must.EqError(t, err, tc.expectErr)
				must.Eq(t, "", authToken)
			} else {
				must.NoError(t, err)
				must.Eq(t, tc.token, authToken)
			}
			must.Eq(t, tc.handshake, called)
		})
	}
}
