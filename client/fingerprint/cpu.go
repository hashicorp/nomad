package fingerprint

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/cpu"
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewCPUFingerprint is used to create a CPU fingerprint
func NewCPUFingerprint(logger *log.Logger) Fingerprint {
	f := &CPUFingerprint{logger: logger}
	return f
}

func (f *CPUFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	cpuInfo, err := cpu.Info()
	if err != nil {
		f.logger.Println("[WARN] Error reading CPU information:", err)
		return false, err
	}

	var mhz float64
	var modelName string

	// Assume all CPUs found have same Model. Log if not. If cpu.Info()
	// returns nil above, this loop is still safe.  Don't assume all
	// platforms return one entry in cpuInfo per core.
	for _, c := range cpuInfo {
		mhz += c.Mhz

		if modelName != "" && modelName != c.ModelName {
			f.logger.Println("[WARN] Found different model names in the same CPU information. Recording last found")
		}
		modelName = c.ModelName
	}
	// Get average CPU frequency
	mhz /= float64(len(cpuInfo))

	// Allow for a little precision slop
	if mhz < 1.0 {
		f.logger.Println("[WARN] fingerprint.cpu: Unable to obtain the CPU Mhz")
	} else {
		node.Attributes["cpu.frequency"] = fmt.Sprintf("%.6f", mhz)
		f.logger.Printf("[DEBUG] fingerprint.cpu: frequency: %02.1f MHz", mhz)
	}

	var numCores int
	if numCores, err = cpu.Counts(true); err != nil {
		numCores = 1
		f.logger.Println("[WARN] Unable to obtain the number of CPUs, defaulting to %d CPU", numCores)
	}

	if numCores > 0 {
		node.Attributes["cpu.numcores"] = fmt.Sprintf("%d", numCores)
		f.logger.Printf("[DEBUG] fingerprint.cpu: core count: %d", numCores)
	}

	if mhz > 0 && numCores > 0 {
		tc := float64(numCores) * mhz
		node.Attributes["cpu.totalcompute"] = fmt.Sprintf("%.6f", tc)

		if node.Resources == nil {
			node.Resources = &structs.Resources{}
		}

		node.Resources.CPU = int(tc)
	}

	if modelName != "" {
		node.Attributes["cpu.modelname"] = modelName
	}

	return true, nil
}
