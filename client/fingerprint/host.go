package fingerprint

import (
	"log"
	"runtime"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/shirou/gopsutil/host"
)

// HostFingerprint is used to fingerprint the host
type HostFingerprint struct {
	StaticFingerprinter
	logger *log.Logger
}

// NewHostFingerprint is used to create a Host fingerprint
func NewHostFingerprint(logger *log.Logger) Fingerprint {
	f := &HostFingerprint{logger: logger}
	return f
}

func (f *HostFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	hostInfo, err := host.Info()
	if err != nil {
		f.logger.Println("[WARN] Error retrieving host information: ", err)
		return err
	}

	resp.AddAttribute("os.name", hostInfo.Platform)
	resp.AddAttribute("os.version", hostInfo.PlatformVersion)

	resp.AddAttribute("kernel.name", runtime.GOOS)
	resp.AddAttribute("kernel.version", hostInfo.KernelVersion)

	resp.AddAttribute("unique.hostname", hostInfo.Hostname)
	resp.Detected = true

	return nil
}
