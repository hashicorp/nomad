package fingerprint

import (
	"fmt"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/stats"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// defaultCPUTicks is the default amount of CPU resources assumed to be
	// available if the CPU performance data is unable to be detected. This is
	// common on EC2 instances, where the env_aws fingerprinter will follow up,
	// setting an accurate value.
	defaultCPUTicks = 1000 // 1 core * 1 GHz
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

	// If we cannot detect the cpu total compute, fallback to a very low default
	// value and log a message about configuring cpu_total_compute. This happens
	// on Graviton instances where CPU information is unavailable. In that case,
	// the env_aws fingerprinter updates the value with correct information.
	if tt == 0 {
		f.logger.Info("fallback to default cpu total compute, set client config option cpu_total_compute to override")
		tt = defaultCPUTicks
	}

	resp.AddAttribute("cpu.totalcompute", fmt.Sprintf("%d", tt))
	setResourcesCPU(tt)
	resp.Detected = true

	return nil
}
