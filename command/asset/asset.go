// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package asset

import _ "embed"

//go:embed example.nomad.hcl
var JobExample []byte

//go:embed example-short.nomad.hcl
var JobExampleShort []byte

//go:embed connect.nomad.hcl
var JobConnect []byte

//go:embed connect-short.nomad.hcl
var JobConnectShort []byte

//go:embed pool.nomad.hcl
var NodePoolSpec []byte

//go:embed pool.nomad.json
var NodePoolSpecJSON []byte
