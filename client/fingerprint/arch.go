package fingerprint

import (
	"os/exec"
	"runtime"
	"strings"

	log "github.com/hashicorp/go-hclog"
)

// ArchFingerprint is used to fingerprint the architecture
type ArchFingerprint struct {
	StaticFingerprinter
	logger log.Logger
}

// NewArchFingerprint is used to create an OS fingerprint
func NewArchFingerprint(logger log.Logger) Fingerprint {
	f := &ArchFingerprint{logger: logger.Named("arch")}
	return f
}

func (f *ArchFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	resp.AddAttribute("cpu.arch", runtime.GOARCH)

	if runtime.GOOS == "linux" {
		unameOutput, err := exec.Command("uname", "-m").Output()

		if err != nil {
			f.logger.Warn("error calling 'uname -m'")
			resp.AddAttribute("cpu.machine", "undefined")
		} else {
			output := strings.TrimSpace(string(unameOutput))
			resp.AddAttribute("cpu.machine", output)
		}
	}

	resp.Detected = true
	return nil
}
