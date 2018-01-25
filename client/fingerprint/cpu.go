package fingerprint

import (
	"fmt"
	"log"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/stats"
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

func (f *CPUFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	cfg := req.Config
	setResources := func(totalCompute int) {
		resp.Resources.CPU = totalCompute
	}

	if err := stats.Init(); err != nil {
		f.logger.Printf("[WARN] fingerprint.cpu: %v", err)
	}

	if cfg.CpuCompute != 0 {
		setResources(cfg.CpuCompute)
		return nil
	}

	if modelName := stats.CPUModelName(); modelName != "" {
		resp.Attributes["cpu.modelname"] = modelName
	}

	if mhz := stats.CPUMHzPerCore(); mhz > 0 {
		resp.Attributes["cpu.frequency"] = fmt.Sprintf("%.0f", mhz)
		f.logger.Printf("[DEBUG] fingerprint.cpu: frequency: %.0f MHz", mhz)
	}

	if numCores := stats.CPUNumCores(); numCores > 0 {
		resp.Attributes["cpu.numcores"] = fmt.Sprintf("%d", numCores)
		f.logger.Printf("[DEBUG] fingerprint.cpu: core count: %d", numCores)
	}

	tt := int(stats.TotalTicksAvailable())
	if cfg.CpuCompute > 0 {
		f.logger.Printf("[DEBUG] fingerprint.cpu: Using specified cpu compute %d", cfg.CpuCompute)
		tt = cfg.CpuCompute
	}

	// Return an error if no cpu was detected or explicitly set as this
	// node would be unable to receive any allocations.
	if tt == 0 {
		return fmt.Errorf("cannot detect cpu total compute. "+
			"CPU compute must be set manually using the client config option %q",
			"cpu_total_compute")
	}

	resp.Attributes["cpu.totalcompute"] = fmt.Sprintf("%d", tt)
	resp.Resources.CPU = tt

	return nil
}
