package fingerprint

import (
	"log"
	"strings"

	"github.com/hashicorp/consul-template/signals"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
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

func (f *SignalFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	// Build the list of available signals
	sigs := make([]string, 0, len(signals.SignalLookup))
	for signal := range signals.SignalLookup {
		sigs = append(sigs, signal)
	}

	node.Attributes["os.signals"] = strings.Join(sigs, ",")
	return true, nil
}
