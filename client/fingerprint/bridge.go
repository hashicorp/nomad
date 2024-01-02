// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import log "github.com/hashicorp/go-hclog"

type BridgeFingerprint struct {
	StaticFingerprinter

	logger log.Logger
}

func NewBridgeFingerprint(logger log.Logger) Fingerprint {
	return &BridgeFingerprint{logger: logger}
}
