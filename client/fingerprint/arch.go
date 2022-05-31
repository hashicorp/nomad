package fingerprint

import (
	"runtime"

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

	if runtime.GOARCH == "amd64" {
		resp.AddAttribute("cpu.arch_classic", "x86_64")
	} else if runtime.GOARCH == "386" {
		resp.AddAttribute("cpu.arch_classic", "i386")
	} else if runtime.GOARCH == "arm64" {
		resp.AddAttribute("cpu.arch_classic", "aarch64")
	}

	resp.Detected = true
	return nil
}
