package fingerprint

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
)

// CPUFingerprint is used to fingerprint the CPU
type CPUFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewCPUFingerprint is used to create a CPU fingerprint
func NewCPUFingerprint(logger log.Logger) Fingerprint {
	f := &CPUFingerprint{logger: logger.Named("cpu")}
	return f
}

func (f *CPUFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	cfg := req.Config
	setResourcesCPU := func(totalCompute int) {
		// COMPAT(0.10): Remove in 0.10
		resp.Resources = &structs.Resources{
			CPU: totalCompute,
		}

		resp.NodeResources = &structs.NodeResources{
			Cpu: structs.NodeCpuResources{
				CpuShares: int64(totalCompute),
			},
		}
	}

	if err := stats.Init(); err != nil {
		f.logger.Warn("failed initializing stats collector", "error", err)
	}

	if cfg.CpuCompute != 0 {
		setResourcesCPU(cfg.CpuCompute)
		return nil
	}

	if modelName := stats.CPUModelName(); modelName != "" {
		resp.AddAttribute("cpu.modelname", modelName)
	}

	if mhz := stats.CPUMHzPerCore(); mhz > 0 {
		resp.AddAttribute("cpu.frequency", fmt.Sprintf("%.0f", mhz))
		f.logger.Debug("detected cpu frequency", "MHz", log.Fmt("%.0f", mhz))
	}

	if numCores := stats.CPUNumCores(); numCores > 0 {
		resp.AddAttribute("cpu.numcores", fmt.Sprintf("%d", numCores))
		f.logger.Debug("detected core count", "cores", numCores)
	}

	tt := int(stats.TotalTicksAvailable())
	if cfg.CpuCompute > 0 {
		f.logger.Debug("using user specified cpu compute", "cpu_compute", cfg.CpuCompute)
		tt = cfg.CpuCompute
	}

	// Return an error if no cpu was detected or explicitly set as this
	// node would be unable to receive any allocations.
	if tt == 0 {
		return fmt.Errorf("cannot detect cpu total compute. "+
			"CPU compute must be set manually using the client config option %q",
			"cpu_total_compute")
	}

	resp.AddAttribute("cpu.totalcompute", fmt.Sprintf("%d", tt))
	setResourcesCPU(tt)
	resp.Detected = true

	return nil
}
