package fingerprint

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/shoenig/go-landlock"
)

const (
	landlockKey = "kernel.landlock"
)

// LandlockFingerprint is used to fingerprint the kernal landlock feature.
type LandlockFingerprint struct {
	StaticFingerprinter
	logger hclog.Logger
}

func NewLandlockFingerprint(logger hclog.Logger) Fingerprint {
	return &LandlockFingerprint{logger: logger.Named("landlock")}
}

func (f *LandlockFingerprint) Fingerprint(_ *FingerprintRequest, resp *FingerprintResponse) error {
	version, err := landlock.Detect()
	if err != nil {
		f.logger.Warn("failed to fingerprint kernel landlock feature", "error", err)
		version = 0
	}
	switch version {
	case 0:
		resp.AddAttribute(landlockKey, "unavailable")
	default:
		v := fmt.Sprintf("v%d", version)
		resp.AddAttribute(landlockKey, v)
	}
	return nil
}
