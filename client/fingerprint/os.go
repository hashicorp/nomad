package fingerprint

import (
	"log"
	"runtime"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/cpu"
)

// OSFingerprint is used to fingerprint the operating system
type OSFingerprint struct {
	logger *log.Logger
}

// NewOSFingerprint is used to create an OS fingerprint
func NewOSFingerprint(logger *log.Logger) Fingerprint {
	f := &OSFingerprint{logger}
	return f
}

func (f *OSFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	node.Attributes["os"] = runtime.GOOS
	f.logger.Printf("[DEBUG] fingerprint.os: detected '%s'", runtime.GOOS)

	cpuInfo, err := cpu.CPUInfo()
	if err != nil {
		f.logger.Println("[WARN] Error reading CPU information:", err)
	}

	var numCores int32
	var modelName string
	// Assume all CPUs found have same Model. Log if not.
	for i, c := range cpuInfo {
		f.logger.Printf("(%d) Vendor: %s", i, c.VendorID)
		numCores += c.Cores
		if modelName != "" && modelName != c.ModelName {
			f.logger.Println("[WARN] Found different model names in the same CPU information. Recording last found")
		}
		modelName = c.ModelName
	}
	if numCores > 0 {
		node.Attributes["cpu.numcores"] = string(numCores)
	}
	if modelName != "" {
		node.Attributes["cpu.modelname"] = modelName
	}

	return true, nil
}
