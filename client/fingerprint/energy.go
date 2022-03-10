package fingerprint

import (
	"context"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"strconv"
)

const (
	carbonScoreAttr = "energy.carbon_score"
)

// EnergyFingerprint is used to fingerprint for Carbon impact
type EnergyFingerprint struct {
	logger       log.Logger
	energyConfig *config.EnergyConfig
}

// NewEnergyFingerprint is used to create a Consul fingerprint
func NewEnergyFingerprint(logger log.Logger) Fingerprint {
	return &EnergyFingerprint{
		logger: logger.Named("energy"),
	}
}

func (f *EnergyFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	// initialize config if necessary
	if f.energyConfig == nil {
		f.energyConfig = req.Config.EnergyConfig
	}

	f.clearEnergyAttributes(resp)

	if f.energyConfig == nil {
		f.logger.Trace("no energy configuration detected")
		return nil
	}

	provider := *f.energyConfig.ScoreProvider
	if provider == nil {
		f.logger.Trace("no energy provider configured")
		return nil
	}

	carbonScore, err := provider.GetCarbonIntensity(context.Background())
	if err != nil {
		return err
	}

	f.logger.Trace("energy.carbon_score", carbonScore)
	resp.AddAttribute(carbonScoreAttr, strconv.Itoa(carbonScore))

	resp.Detected = true

	return nil
}

func (f *EnergyFingerprint) Periodic() (bool, time.Duration) {
	return true, 5 * time.Minute
}

// clearEnergyAttributes removes consul attributes and links from the passed Node.
func (f *EnergyFingerprint) clearEnergyAttributes(r *FingerprintResponse) {
	r.RemoveAttribute(carbonScoreAttr)
}
