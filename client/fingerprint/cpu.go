package fingerprint

import (
	"fmt"
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
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
	setResources := func(totalCompute int) {
		if node.Resources == nil {
			node.Resources = &structs.Resources{}
		}

		node.Resources.CPU = totalCompute
	}

	if err := stats.Init(); err != nil {
		f.logger.Printf("[WARN] fingerprint.cpu: %v", err)
	}

	if cfg.CpuCompute != 0 {
		setResources(cfg.CpuCompute)
		return true, nil
	}

	if modelName := stats.CPUModelName(); modelName != "" {
		node.Attributes["cpu.modelname"] = modelName
	}

	if mhz := stats.CPUMHzPerCore(); mhz > 0 {
		node.Attributes["cpu.frequency"] = fmt.Sprintf("%.0f", mhz)
		f.logger.Printf("[DEBUG] fingerprint.cpu: frequency: %.0f MHz", mhz)
	}

	if numCores := stats.CPUNumCores(); numCores > 0 {
		node.Attributes["cpu.numcores"] = fmt.Sprintf("%d", numCores)
		f.logger.Printf("[DEBUG] fingerprint.cpu: core count: %d", numCores)
	}

	tt := int(stats.TotalTicksAvailable())
	if cfg.CpuCompute > 0 {
		f.logger.Printf("[DEBUG] fingerprint.cpu: Using specified cpu compute %d", cfg.CpuCompute)
		tt = cfg.CpuCompute
	}

	// Set the node compute resources if they are detected/configured
	if tt > 0 {
		node.Attributes["cpu.totalcompute"] = fmt.Sprintf("%d", tt)

		if node.Resources == nil {
			node.Resources = &structs.Resources{}
		}

		node.Resources.CPU = tt
	} else {
		f.logger.Printf("[INFO] fingerprint.cpu: cannot detect cpu total compute. "+
			"CPU compute must be set manually using the client config option %q",
			"cpu_total_compute")
	}
	return true, nil
}
