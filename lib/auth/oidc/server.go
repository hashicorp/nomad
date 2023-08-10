// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
		url:         fmt.Sprintf("http://%s/oidc/callback", addr),
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
// request to complete the OIDC auth with the Nomad server.
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
	_, _ = w.Write([]byte(serverSuccessHTMLResponse))
}

// serverSuccessHTMLResponse is the HTML response the OIDC callback server uses
// when the user has successfully logged in via the OIDC provider.
const serverSuccessHTMLResponse = `
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>OIDC Authentication Succeeded</title>
    <style>
      body {
        font-size: 14px;
        font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI",
          "Roboto", "Oxygen", "Ubuntu", "Cantarell", "Fira Sans", "Droid Sans",
          "Helvetica Neue", sans-serif;
      }
      hr {
        border-color: #fdfdfe;
        margin: 24px 0;
      }
      .container {
        display: flex;
        justify-content: center;
        align-items: center;
        height: 70vh;
      }
      svg.logo {
        display: block;
        margin: 0 0 20px;
      }
      .message {
        display: flex;
        min-width: 40vw;
        background: #f0f5ff;
        border: 1px solid #1563ff;
        margin-bottom: 12px;
        padding: 12px 16px 16px 12px;
        position: relative;
        border-radius: 2px;
        font-size: 14px;
        color: #0a2d74;
      }
      .message-content {
        margin-left: 4px;
      }
      .message #checkbox {
        fill: #1563ff;
      }
      .message .message-title {
        font-size: 16px;
        font-weight: 700;
        line-height: 1.25;
      }
      .message .message-body {
        border: 0;
        margin-top: 4px;
      }
      .message p {
        font-size: 12px;
        margin: 0;
        padding: 0;
      }
      a {
        display: block;
        margin: 8px 0;
        color: #1563ff;
        text-decoration: none;
        font-weight: 600;
      }
      a:hover {
        color: black;
      }
      a svg {
        fill: currentcolor;
      }
      .icon {
        align-items: center;
        display: inline-flex;
        justify-content: center;
        height: 21px;
        width: 21px;
        vertical-align: middle;
      }
      h1 {
        font-size: 17.5px;
        font-weight: 700;
        margin-bottom: 0;
      }
      h1 + p {
        margin: 8px 0 16px 0;
      }
    </style>
  </head>
  <body translate="no" >
    <div class="container">
      <div>
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="20 0 342 107" id="LOGOS" height="50">
          <defs><style>.cls-1{fill:#00ca8e;}</style></defs>
          <path d="M125,61.42V92h-7.44V51.68h10.16L143,82.29V51.68h7.44V92H140.28Z"></path>
          <path d="M168.58,92.58c-10.1,0-12.82-5.57-12.82-11.62V73.52c0-6,2.72-11.61,12.82-11.61s12.83,5.56,12.83,11.61V81C181.41,87,178.69,92.58,168.58,92.58Zm0-24.38c-3.93,0-5.44,1.75-5.44,5.08V81.2c0,3.33,1.51,5.09,5.44,5.09S174,84.53,174,81.2V73.28C174,70,172.52,68.2,168.58,68.2Z"></path>
          <path d="M203.85,92V71.4c0-1.57-.67-2.36-2.36-2.36s-5,1.09-7.68,2.48V92h-7.38V62.51h5.62l.73,2.48a29.59,29.59,0,0,1,11.8-3.08c2.84,0,4.59,1.15,5.56,3.14A29,29,0,0,1,222,61.91c4.9,0,6.65,3.44,6.65,8.71V92h-7.38V71.4c0-1.57-.66-2.36-2.36-2.36a19.55,19.55,0,0,0-7.68,2.48V92Z"></path>
          <path d="M256.54,92h-6.05l-.55-2a16.15,16.15,0,0,1-8.77,2.6c-5.38,0-7.68-3.69-7.68-8.77,0-6,2.6-8.29,8.59-8.29h7.08V72.43c0-3.26-.91-4.41-5.63-4.41a41.55,41.55,0,0,0-8.17.9l-.9-5.62a38.26,38.26,0,0,1,10.1-1.39c9.25,0,12,3.26,12,10.64Zm-7.38-11.13h-5.45c-2.42,0-3.08.67-3.08,2.91,0,2,.66,3,3,3a11.69,11.69,0,0,0,5.57-1.51Z"></path>
          <path d="M261.07,72.31c0-6.53,2.9-10.4,9.74-10.4a41.09,41.09,0,0,1,7.87.84V50.47l7.38-1V92h-5.87l-.73-2.48a15.46,15.46,0,0,1-9.31,3.09c-5.93,0-9.08-3.51-9.08-10.23ZM278.68,69a33.06,33.06,0,0,0-6.54-.78c-2.66,0-3.69,1.27-3.69,3.93V82.54c0,2.42.91,3.75,3.63,3.75a10.43,10.43,0,0,0,6.6-2.67Z"></path>
          <path class="cls-1" d="M61.14,38.19,32,55V88.63l29.12,16.81L90.26,88.63V55Zm13,37-7.76,4.48L57,74.54V85.26l-8.81,5.59V68.45l7-4.28,9.7,5.11V58.34l9.25-5.56Z"></path>
          <path d="M125.42,41.81V36.15h-5.17v5.66h-2.64V28.22h2.64v5.7h5.17v-5.7h2.65V41.81Zm12.33,0h-2.1l-.19-.67a5.69,5.69,0,0,1-3,.87c-1.86,0-2.66-1.23-2.66-2.92,0-2,.9-2.76,3-2.76h2.45v-1c0-1.09-.31-1.47-1.95-1.47a14.94,14.94,0,0,0-2.83.3l-.31-1.87a13.9,13.9,0,0,1,3.5-.46c3.2,0,4.15,1.08,4.15,3.54Zm-2.56-3.7H133.3c-.83,0-1.06.22-1.06,1s.23,1,1,1a4,4,0,0,0,1.93-.51ZM143.1,42a12.59,12.59,0,0,1-3.52-.56l.36-1.88a11.5,11.5,0,0,0,3,.43c1.13,0,1.3-.24,1.3-1s-.13-.9-1.78-1.29c-2.5-.58-2.79-1.18-2.79-3.08s.9-2.83,3.81-2.83a14,14,0,0,1,3.06.34l-.25,2a18.71,18.71,0,0,0-2.81-.28c-1.11,0-1.3.24-1.3.84,0,.79.07.85,1.45,1.19,2.85.73,3.12,1.09,3.12,3.1S146.18,42,143.1,42Zm11.71-.2V35c0-.53-.23-.79-.82-.79a7.12,7.12,0,0,0-2.66.83v6.8h-2.56V28l2.56.38v4.34a9.34,9.34,0,0,1,3.73-.94c1.7,0,2.31,1.14,2.31,2.89v7.11Zm4.7-11.19v-2.4h2.56v2.4Zm0,11.19V32h2.56v9.8Zm4.6-9.72c0-2.46,1.49-3.88,5-3.88a16.47,16.47,0,0,1,3.79.44l-.29,2.19a21.49,21.49,0,0,0-3.42-.34c-1.82,0-2.41.6-2.41,2v5.15c0,1.43.59,2,2.41,2a21.57,21.57,0,0,0,3.42-.35l.29,2.2a16.47,16.47,0,0,1-3.79.44c-3.48,0-5-1.43-5-3.88ZM178.52,42c-3.5,0-4.44-1.85-4.44-3.86V35.67c0-2,.94-3.86,4.44-3.86S183,33.66,183,35.67v2.48C183,40.16,182,42,178.52,42Zm0-8.11c-1.36,0-1.89.58-1.89,1.69v2.64c0,1.1.53,1.69,1.89,1.69s1.89-.59,1.89-1.69V35.59C180.41,34.48,179.88,33.9,178.52,33.9Zm11.64.16a20.53,20.53,0,0,0-2.7,1.43v6.32H184.9V32h2.16l.17,1.08a11.54,11.54,0,0,1,2.68-1.28Zm10.22,4.49c0,2.17-1,3.46-3.37,3.46a14.85,14.85,0,0,1-2.73-.28v4l-2.55.38V32h2l.25.82a5.54,5.54,0,0,1,3.23-1c2,0,3.14,1.16,3.14,3.4Zm-6.1,1.11a12,12,0,0,0,2.27.26c.92,0,1.28-.43,1.28-1.31V35.15c0-.81-.32-1.25-1.26-1.25a3.68,3.68,0,0,0-2.29.89Z"></path>
        </svg>
        <div class="message">
         <svg id="checkbox" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 16 16" fill="currentColor">
              <path fill-rule="evenodd" d="M8 16A8 8 0 108 0a8 8 0 000 16zm.93-9.412l-2.29.287-.082.38.45.083c.294.07.352.176.288.469l-.738 3.468c-.194.897.105 1.319.808 1.319.545 0 1.178-.252 1.465-.598l.088-.416c-.2.176-.492.246-.686.246-.275 0-.375-.193-.304-.533L8.93 6.588zM8 5.5a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/>
          </svg>
          <div class="message-content">
            <div class="message-title">
              Signed in via your OIDC provider
            </div>
            <p class="message-body">
              You can now close this window and start using Nomad.
            </p>
          </div>
        </div>
        <hr />
        <h1>Not sure how to get started?</h1>
        <p class="learn">
          Check out beginner and advanced guides on HashiCorp Nomad at the HashiCorp Learn site or read more in the official documentation.
        </p>
        <a href="https://developer.hashicorp.com/nomad/tutorials" rel="noreferrer noopener">
         <span class="icon">
            <svg width="16" height="16" viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">
              <path d="M8.338 2.255a.79.79 0 0 0-.645 0L.657 5.378c-.363.162-.534.538-.534.875 0 .337.171.713.534.875l1.436.637c-.332.495-.638 1.18-.744 2.106a.887.887 0 0 0-.26 1.559c.02.081.03.215.013.392-.02.205-.074.43-.162.636-.186.431-.45.64-.741.64v.98c.651 0 1.108-.365 1.403-.797l.06.073c.32.372.826.763 1.455.763v-.98c-.215 0-.474-.145-.71-.42-.111-.13-.2-.27-.259-.393a1.014 1.014 0 0 1-.06-.155c-.01-.036-.013-.055-.013-.058h-.022a2.544 2.544 0 0 0 .031-.641.886.886 0 0 0-.006-1.51c.1-.868.398-1.477.699-1.891l.332.147-.023.746v2.228c0 .115.04.22.105.304.124.276.343.5.587.677.297.217.675.396 1.097.54.846.288 1.943.456 3.127.456 1.185 0 2.281-.168 3.128-.456.422-.144.8-.323 1.097-.54.244-.177.462-.401.586-.677a.488.488 0 0 0 .106-.304V8.218l2.455-1.09c.363-.162.534-.538.534-.875 0-.337-.17-.713-.534-.875L8.338 2.255zm-.34 2.955L3.64 7.38l4.375 1.942 6.912-3.069-6.912-3.07-6.912 3.07 1.665.74 4.901-2.44.328.657zM14.307 1H12.5a.5.5 0 1 1 0-1h3a.499.499 0 0 1 .5.65V3.5a.5.5 0 1 1-1 0V1.72l-1.793 1.774a.5.5 0 0 1-.713-.701L14.307 1zm-2.368 7.653v2.383a.436.436 0 0 0-.007.021c-.017.063-.084.178-.282.322-.193.14-.473.28-.836.404-.724.247-1.71.404-2.812.404-1.1 0-2.087-.157-2.811-.404a3.188 3.188 0 0 1-.836-.404c-.198-.144-.265-.26-.282-.322a.437.437 0 0 0-.007-.02V8.983l.01-.338 3.617 1.605a.791.791 0 0 0 .645 0l3.6-1.598z" fill-rule="evenodd"></path>
            </svg>
          </span>
          Get started with Nomad
        </a>
        <a href="https://developer.hashicorp.com/nomad/docs" rel="noreferrer noopener">
         <span class="icon">
          <svg width="16" height="16" viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">
    <path d="M13.307 1H11.5a.5.5 0 1 1 0-1h3a.499.499 0 0 1 .5.65V3.5a.5.5 0 1 1-1 0V1.72l-1.793 1.774a.5.5 0 0 1-.713-.701L13.307 1zM12 14V8a.5.5 0 1 1 1 0v6.5a.5.5 0 0 1-.5.5H.563a.5.5 0 0 1-.5-.5v-13a.5.5 0 0 1 .5-.5H8a.5.5 0 0 1 0 1H1v12h11zM4 6a.5.5 0 0 1 0-1h3a.5.5 0 0 1 0 1H4zm0 2.5a.5.5 0 0 1 0-1h5a.5.5 0 0 1 0 1H4zM4 11a.5.5 0 1 1 0-1h5a.5.5 0 1 1 0 1H4z"/>
  </svg> 
          </span>
          View the official Nomad documentation
        </a>
      </div>
    </div>
  </body>
</html>
`
