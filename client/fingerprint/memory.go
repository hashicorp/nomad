package fingerprint

import (
	"fmt"
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/mem"
)

// MemoryFingerprint is used to fingerprint the available memory on the node
type MemoryFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewMemoryFingerprint is used to create a Memory fingerprint
func NewMemoryFingerprint(logger *log.Logger) Fingerprint {
	f := &MemoryFingerprint{
		logger: logger,
	}
	return f
}

func (f *MemoryFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		f.logger.Printf("[WARN] Error reading memory information: %s", err)
		return err
	}

	if memInfo.Total > 0 {
		resp.AddAttribute("memory.totalbytes", fmt.Sprintf("%d", memInfo.Total))

		resp.Resources = &structs.Resources{
			MemoryMB: int(memInfo.Total / 1024 / 1024),
		}
	}

	return nil
}
