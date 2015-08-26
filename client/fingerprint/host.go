package fingerprint

import (
	"log"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shirou/gopsutil/host"
)

// HostFingerprint is used to fingerprint the host
type HostFingerprint struct {
	logger *log.Logger
}

// NewHostFingerprint is used to create a Host fingerprint
func NewHostFingerprint(logger *log.Logger) Fingerprint {
	f := &HostFingerprint{logger}
	return f
}

func (f *HostFingerprint) Fingerprint(cfg *config.Config, node *structs.Node) (bool, error) {
	hostInfo, err := host.HostInfo()
	if err != nil {
		f.logger.Println("[WARN] Error retrieving host information: ", err)
		return false, err
	}

	node.Attributes["os.name"] = hostInfo.Platform
	node.Attributes["os.version"] = hostInfo.PlatformVersion
	node.Attributes["hostname"] = hostInfo.Hostname
	node.Attributes["kernel.name"] = hostInfo.OS

	return true, nil
}
