// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/lib/numalib"
	"github.com/hashicorp/nomad/nomad/structs"
)

// FingerprintRequest is a request which a fingerprinter accepts to fingerprint
// the node
type FingerprintRequest struct {
	Config *config.Config
	Node   *structs.Node
}

// FingerprintResponse is the response which a fingerprinter annotates with the
// results of the fingerprint method
type FingerprintResponse struct {
	Attributes    map[string]string
	Links         map[string]string
	Resources     *structs.Resources // COMPAT(0.10): Remove in 0.10
	NodeResources *structs.NodeResources

	// Detected is a boolean indicating whether the fingerprinter detected
	// if the resource was available
	Detected bool

	// UpdateInitialResult can be used by a fingerprinter to pass information it
	// discovers back up to the client.
	UpdateInitialResult func(*InitialResult)
}

// InitialResult can be updated by individual fingerprinters to bubble information
// it detected back up to the client after the first pass.
type InitialResult struct {
	Topology *numalib.Topology
}

// AddAttribute adds the name and value for a node attribute to the fingerprint
// response
func (f *FingerprintResponse) AddAttribute(name, value string) {
	// initialize Attributes if it has not been already
	if f.Attributes == nil {
		f.Attributes = make(map[string]string)
	}

	f.Attributes[name] = value
}

// RemoveAttribute sets the given attribute to empty, which will later remove
// it entirely from the node
func (f *FingerprintResponse) RemoveAttribute(name string) {
	// initialize Attributes if it has not been already
	if f.Attributes == nil {
		f.Attributes = make(map[string]string)
	}

	f.Attributes[name] = ""
}

// AddLink adds a link entry to the fingerprint response
func (f *FingerprintResponse) AddLink(name, value string) {
	// initialize Links if it has not been already
	if f.Links == nil {
		f.Links = make(map[string]string)
	}

	f.Links[name] = value
}

// RemoveLink removes a link entry from the fingerprint response. This will
// later remove it entirely from the node
func (f *FingerprintResponse) RemoveLink(name string) {
	// initialize Links if it has not been already
	if f.Links == nil {
		f.Links = make(map[string]string)
	}

	f.Links[name] = ""
}
