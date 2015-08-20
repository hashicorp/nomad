package fingerprint

import (
	"log"
	"runtime"

	"github.com/hashicorp/nomad/nomad/structs"
)

// ArchFingerprint is used to fingerprint the architecture
type ArchFingerprint struct {
	logger *log.Logger
}

// NewArchFingerprint is used to create an OS fingerprint
func NewArchFingerprint(logger *log.Logger) Fingerprint {
	f := &ArchFingerprint{logger}
	return f
}

func (f *ArchFingerprint) Fingerprint(node *structs.Node) (bool, error) {
	node.Attributes["arch"] = runtime.GOARCH
	f.logger.Printf("[DEBUG] fingerprint.arch: detected '%s'", runtime.GOARCH)
	return true, nil
}
