// Copyright 2015 go-dockerclient authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import "strings"

// Version returns version information about the docker server.
//
// See https://goo.gl/ND9R8L for more details.
func (c *Client) Version() (*Env, error) {
	resp, err := c.do("GET", "/version", doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var env Env
	if err := env.Decode(resp.Body); err != nil {
		return nil, err
	}
	return &env, nil
}

// Info returns system-wide information about the Docker server.
//
// See https://goo.gl/ElTHi2 for more details.
func (c *Client) Info() (*Env, error) {
	resp, err := c.do("GET", "/info", doOptions{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var info Env
	if err := info.Decode(resp.Body); err != nil {
		return nil, err
	}
	return &info, nil
}

// ParseRepositoryTag gets the name of the repository and returns it splitted
// in two parts: the repository and the tag.
//
// Some examples:
//
//     localhost.localdomain:5000/samalba/hipache:latest -> localhost.localdomain:5000/samalba/hipache, latest
//     localhost.localdomain:5000/samalba/hipache -> localhost.localdomain:5000/samalba/hipache, ""
func ParseRepositoryTag(repoTag string) (repository string, tag string) {
	n := strings.LastIndex(repoTag, ":")
	if n < 0 {
		return repoTag, ""
	}
	if tag := repoTag[n+1:]; !strings.Contains(tag, "/") {
		return repoTag[:n], tag
	}
	return repoTag, ""
}
