package fingerprint

import log "github.com/hashicorp/go-hclog"

type BridgeFingerprint struct {
	logger log.Logger
	StaticFingerprinter
}

func NewBridgeFingerprint(logger log.Logger) Fingerprint {
	return &BridgeFingerprint{logger: logger}
}
