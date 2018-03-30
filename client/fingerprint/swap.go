package fingerprint

import (
	"fmt"
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
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

func (f *SwapFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	swapInfo, err := mem.SwapMemory()
	if err != nil {
		f.logger.Printf("[WARN] Error reading swap information: %s", err)
		return err
	}

	if swapInfo.Total > 0 {
		resp.AddAttribute("swap.totalbytes", fmt.Sprintf("%d", swapInfo.Total))

		resp.Resources = &structs.Resources{
			SwapMB: int(swapInfo.Total / 1024 / 1024),
		}
	}

	return nil
}
