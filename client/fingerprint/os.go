package fingerprint

import (
	"log"
	"runtime"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// OSFingerprint is used to fingerprint the operating system
type OSFingerprint struct {
	id     string
	logger *log.Logger
}

// NewOSFingerprint is used to create an OS fingerprint
func NewOSFingerprint(logger *log.Logger) Fingerprint {
	f := &OSFingerprint{
		id:     "fingerprint.os",
		logger: logger,
	}
	return f
}

func (f *OSFingerprint) ID() string {
	return f.id
}

func (f *OSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	node.Attributes["os"] = runtime.GOOS
	f.logger.Printf("[DEBUG] fingerprint.os: detected '%s'", runtime.GOOS)

	return true, nil
}
