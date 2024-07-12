package fingerprint

import (
	"errors"
	"fmt"
	"time"

	log "github.com/hashicorp/go-hclog"
)

const (
	energyProviderAvailable   = "available"
	energyProviderUnavailable = "unavailable"
)

type EnergyFingerprintProvider interface {
	GetResponse() (EnergyFingerprintResponse, error)
}

type EnergyFingerprintResponse struct {
	CarbonScore int
}

func (ef *EnergyFingerprintResponse) ToFingerprintResponse() (*FingerprintResponse, error) {
	return nil, errors.New("not implemented")
}

// EnergyFingerprint is used to fingerprint the node's carbon score.
type EnergyFingerprint struct {
	logger     log.Logger
	provider   EnergyFingerprintProvider
	lastState  string
	extractors map[string]energyExtractor
}

// energyExtractor is used to parse out one attribute from the EnergyFingerprintProvider.
// Returns the value of the attribute, and whether the attribute exists.
type energyExtractor func(provider EnergyFingerprintProvider) (string, bool)

// NewEnergyFingerprint is used to create an energy fingerprint.
func NewEnergyFingerprint(logger log.Logger) Fingerprint {
	return &EnergyFingerprint{
		logger:    logger.Named("energy"),
		lastState: energyProviderUnavailable,
	}
}

func (f *EnergyFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {

	// establish energy fingerprint provider if enabled and configured
	if err := f.initialize(req); err != nil {
		return err
	}

	// query energy fingerprint provider
	resp, err := f.query()
	if err != nil {
		// unable to reach energy fingerprint provider, nothing to do this time
		return err
	}

	// apply the extractor for each attribute
	for attr, extractor := range f.extractors {
		if s, ok := extractor(f.provider); !ok {
			f.logger.Warn("unable to fingerprint energy profile", "attribute", attr)
		} else {
			resp.AddAttribute(attr, s)
		}
	}

	// create link for energy fingerprint provider
	f.link(resp)

	// indicate energy fingerprint provider is now available
	if f.lastState == energyProviderUnavailable {
		f.logger.Info("energy fingerprint provider is available")
	}

	f.lastState = energyProviderAvailable
	resp.Detected = true
	return nil
}

func (f *EnergyFingerprint) Periodic() (bool, time.Duration) {
	return true, 30 * time.Minute
}

// clearEnergyAttributes removes energy fingerprint provider attributes and links from the passed Node.
func (f *EnergyFingerprint) clearEnergyAttributes(r *FingerprintResponse) {
	for attr := range f.extractors {
		r.RemoveAttribute(attr)
	}
	r.RemoveLink("energy")
}

func (f *EnergyFingerprint) initialize(req *FingerprintRequest) error {
	// Only create the carbon score provider once to avoid creating many connections
	if f.provider == nil {
		var err error
		f.provider, err = newProviderFromOptions(req.Config.Options)
		if err != nil {
			return fmt.Errorf("failed to initialize energy provider: %s", err)
		}

		f.extractors = map[string]energyExtractor{
			"energy.carbon.score": f.carbonScore,
		}
	}

	return nil
}

func newProviderFromOptions(options map[string]string) (EnergyFingerprintProvider, error) {
	return nil, errors.New("not implemented")
}

func (f *EnergyFingerprint) query() (*FingerprintResponse, error) {
	var resp *FingerprintResponse
	providerResp, err := f.provider.GetResponse()
	if err != nil {
		f.handleError(resp)
		return nil, err
	}

	resp, err = providerResp.ToFingerprintResponse()
	if err != nil {
		f.handleError(resp)
		return nil, err
	}
	return resp, nil
}

func (f *EnergyFingerprint) handleError(resp *FingerprintResponse) {
	f.clearEnergyAttributes(resp)

	// indicate energy provider no longer available
	if f.lastState == energyProviderAvailable {
		f.logger.Info("energy provider is unavailable")
	}
	f.lastState = energyProviderUnavailable
}

// TODO: Devise linking implementation if necessary.
func (f *EnergyFingerprint) link(resp *FingerprintResponse) {
	if dc, ok := resp.Attributes["energy"]; ok {
		if name, ok2 := resp.Attributes["unique.consul.name"]; ok2 {
			resp.AddLink("consul", fmt.Sprintf("%s.%s", dc, name))
		}
	} else {
		f.logger.Warn("malformed energy response prevented linking")
	}
}

func (f *EnergyFingerprint) carbonScore(provider EnergyFingerprintProvider) (string, bool) {
	return "not implemented", false
}
