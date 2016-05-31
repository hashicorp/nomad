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

	var numCores int32
	var mhz float64
	var modelName string

	// Assume all CPUs found have same Model. Log if not.
	// If CPUInfo() returns nil above, this loop is still safe
	for _, c := range cpuInfo {
		numCores += c.Cores
		mhz += c.Mhz

		if modelName != "" && modelName != c.ModelName {
			f.logger.Println("[WARN] Found different model names in the same CPU information. Recording last found")
		}
		modelName = c.ModelName
	}
	// Get average CPU frequency
	mhz /= float64(len(cpuInfo))

	if mhz > 0 {
		node.Attributes["cpu.frequency"] = fmt.Sprintf("%.6f", mhz)
		f.logger.Printf("[DEBUG] fingerprint.cpu: frequency: %02.1fMHz", mhz)
	}

	if numCores <= 0 {
		const defaultCPUCoreCount = 1
		f.logger.Printf("[DEBUG] fingerprint.cpu: unable to find core count, defaulting to %d", defaultCPUCoreCount)
		numCores = defaultCPUCoreCount
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
