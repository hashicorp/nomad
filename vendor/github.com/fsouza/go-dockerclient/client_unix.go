// +build !windows
// Copyright 2016 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package docker provides a client for the Docker remote API.
//
// See https://goo.gl/G3plxW for more details on the remote API.
package docker

import (
	"net"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
)

// initializeNativeClient initializes the native Unix domain socket client on
// Unix-style operating systems
func (c *Client) initializeNativeClient() {
	if c.endpointURL.Scheme != unixProtocol {
		return
	}
	socketPath := c.endpointURL.Path
	tr := cleanhttp.DefaultTransport()
	tr.Dial = func(network, addr string) (net.Conn, error) {
		return c.Dialer.Dial(unixProtocol, socketPath)
	}
	c.nativeHTTPClient = &http.Client{Transport: tr}
}
