package fingerprint

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/mem"
)

const bytesInMB = 1024 * 1024

// MemoryFingerprint is used to fingerprint the available memory on the node
type MemoryFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewMemoryFingerprint is used to create a Memory fingerprint
func NewMemoryFingerprint(logger log.Logger) Fingerprint {
	f := &MemoryFingerprint{
		logger: logger.Named("memory"),
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
			f.logger.Warn("error reading memory information", "error", err)
			return err
		}
		if memInfo.Total > 0 {
			totalMemory = int(memInfo.Total)
		}
	}

	if totalMemory > 0 {
		resp.AddAttribute("memory.totalbytes", fmt.Sprintf("%d", totalMemory))

		// COMPAT(0.10): Remove in 0.10
		resp.Resources = &structs.Resources{
			MemoryMB: totalMemory / bytesInMB,
		}

		resp.NodeResources = &structs.NodeResources{
			Memory: structs.NodeMemoryResources{
				MemoryMB: int64(totalMemory / bytesInMB),
			},
		}
	}

	return nil
}
