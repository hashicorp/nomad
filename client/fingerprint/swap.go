package fingerprint

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/mem"
)

// SwapFingerprint is used to fingerprint the available swap on the node
type SwapFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewSwapFingerprint is used to create a Memory fingerprint
func NewSwapFingerprint(logger *log.Logger) Fingerprint {
	f := &SwapFingerprint{
		logger: logger,
	}
	return f
}

func (f *SwapFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	swapInfo, err := mem.SwapMemory()
	if err != nil {
		f.logger.Printf("[WARN] Error reading swap information: %s", err)
		return false, err
	}

	if swapInfo.Total > 0 {
		node.Attributes["swap.total"] = fmt.Sprintf("%d", swapInfo.Total)

		if node.Resources == nil {
			node.Resources = &structs.Resources{}
		}
		node.Resources.SwapMB = int(swapInfo.Total / 1024 / 1024)
    }

	return true, nil
}
