package fingerprint

import (
	"fmt"
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/mem"
)

const bytesInMB = 1024 * 1024

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
	var totalMemory int
	cfg := req.Config
	if cfg.MemoryMB != 0 {
		totalMemory = cfg.MemoryMB * bytesInMB
	} else {
		memInfo, err := mem.VirtualMemory()
		if err != nil {
			f.logger.Printf("[WARN] Error reading memory information: %s", err)
			return err
		}
		if memInfo.Total > 0 {
			totalMemory = int(memInfo.Total)
		}
	}

	if totalMemory > 0 {
		resp.AddAttribute("memory.totalbytes", fmt.Sprintf("%d", totalMemory))
		resp.Resources = &structs.Resources{
			MemoryMB: totalMemory / bytesInMB,
		}
	}

	return nil
}
