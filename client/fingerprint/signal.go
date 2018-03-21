package fingerprint

import (
	"log"
	"strings"

	"github.com/hashicorp/consul-template/signals"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// SignalFingerprint is used to fingerprint the available signals
type SignalFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewSignalFingerprint is used to create a Signal fingerprint
func NewSignalFingerprint(logger *log.Logger) Fingerprint {
	f := &SignalFingerprint{logger: logger}
	return f
}

func (f *SignalFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	// Build the list of available signals
	sigs := make([]string, 0, len(signals.SignalLookup))
	for signal := range signals.SignalLookup {
		sigs = append(sigs, signal)
	}

	resp.AddAttribute("os.signals", strings.Join(sigs, ","))
	resp.Detected = true
	return nil
}
