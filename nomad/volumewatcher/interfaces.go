// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package volumewatcher

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// CSIVolumeRPC is a minimal interface of the Server, intended as an aid
// for testing logic surrounding server-to-server or server-to-client
// RPC calls and to avoid circular references between the nomad
// package and the volumewatcher
type CSIVolumeRPC interface {
	Unpublish(args *structs.CSIVolumeUnpublishRequest, reply *structs.CSIVolumeUnpublishResponse) error
}
