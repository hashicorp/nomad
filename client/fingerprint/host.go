package fingerprint

import (
	"log"
	"runtime"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
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

func (f *HostFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	hostInfo, err := host.Info()
	if err != nil {
		f.logger.Println("[WARN] Error retrieving host information: ", err)
		return false, err
	}

	node.Attributes["os.name"] = hostInfo.Platform
	node.Attributes["os.version"] = hostInfo.PlatformVersion

	node.Attributes["kernel.name"] = runtime.GOOS
	node.Attributes["kernel.version"] = ""

	if runtime.GOOS != "windows" {
		node.Attributes["kernel.version"] = hostInfo.KernelVersion
	}

	node.Attributes["unique.hostname"] = hostInfo.Hostname

	return true, nil
}
