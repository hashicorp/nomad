package fingerprint

import (
	"log"
	"strconv"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/cpu"
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	logger *log.Logger
}

// NewCPUFingerprint is used to create a CPU fingerprint
func NewCPUFingerprint(logger *log.Logger) Fingerprint {
	f := &CPUFingerprint{logger}
	return f
}

func (f *CPUFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	cpuInfo, err := cpu.CPUInfo()
	if err != nil {
		f.logger.Println("[WARN] Error reading CPU information:", err)
		return false, err
	}

	var numCores int32
	numCores = 0
	var modelName string
	// Assume all CPUs found have same Model. Log if not.
	// If CPUInfo() returns nil above, this loop is still safe
	for i, c := range cpuInfo {
		f.logger.Printf("(%d) Vendor: %s", i, c.VendorID)
		numCores += c.Cores
		if modelName != "" && modelName != c.ModelName {
			f.logger.Println("[WARN] Found different model names in the same CPU information. Recording last found")
		}
		modelName = c.ModelName
	}
	if numCores > 0 {
		node.Attributes["cpu.numcores"] = strconv.FormatInt(int64(numCores), 10)
	}
	if modelName != "" {
		node.Attributes["cpu.modelname"] = modelName
	}

	return true, nil
}
