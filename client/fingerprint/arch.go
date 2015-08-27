package fingerprint

import (
	"log"
	"runtime"

	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ArchFingerprint is used to fingerprint the architecture
type ArchFingerprint struct {
	id     string
	logger *log.Logger
}

// NewArchFingerprint is used to create an OS fingerprint
func NewArchFingerprint(logger *log.Logger) Fingerprint {
	f := &ArchFingerprint{
		id:     "fingerprint.arch",
		logger: logger,
	}
	return f
}

func (f *ArchFingerprint) ID() string {
	return f.id
}

func (f *ArchFingerprint) Fingerprint(config *client.Config, node *structs.Node) (bool, error) {
	node.Attributes["arch"] = runtime.GOARCH
	f.logger.Printf("[DEBUG] fingerprint.arch: detected '%s'", runtime.GOARCH)
	return true, nil
}
