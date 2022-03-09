package config

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-multierror"
	grid "github.com/thegreenwebfoundation/grid-intensity-go"
	ci "github.com/thegreenwebfoundation/grid-intensity-go/carbonintensity"
)

// CarbonScoreProviderKey is the discriminator that is used to create determine with concrete
// provider instance to instantiate.
type CarbonScoreProviderKey string

// CarbonScoreProvider is the strategy that returns a CarbonIntensity score for the node.
type CarbonScoreProvider interface {
	GetCarbonIntensity(ctx context.Context) (float64, error)
}

const (
	AWS CarbonScoreProviderKey = "aws"
	GCP CarbonScoreProviderKey = "gcp"
	AZ  CarbonScoreProviderKey = "azure"
	EM  CarbonScoreProviderKey = "electricity-map"
	CI  CarbonScoreProviderKey = "carbon-intensity"
)

// TODO: Should this be agent config and available on servers as well?

// CarbonConfig represents the possible configurations for carbon emissions
// based off parsed client config.
type CarbonConfig struct {
	Region                string                 `hcl:"region"`
	ProviderKey           CarbonScoreProviderKey `hcl:"provider"`
	Provider              *CarbonScoreProvider
	AWSConfig             *AWSConfig             `hcl:"aws"`
	GCPConfig             *GCPConfig             `hcl:"gcp"`
	AzureConfig           *AzureConfig           `hcl:"azure"`
	CarbonIntensityConfig *CarbonIntensityConfig `hcl:"carbon_intensity"`
	ElectricityMapsConfig *ElectricityMapsConfig `hcl:"electricity_map"`
}

func (cc *CarbonConfig) Validate() (err error) {
	if cc == nil {
		return fmt.Errorf("invalid carbon config: config is nil")
	}

	switch cc.ProviderKey {
	case AWS:
		err = cc.AWSConfig.Validate()
	case GCP:
		err = cc.GCPConfig.Validate()
	case AZ:
		err = cc.AzureConfig.Validate()
	case CI:
		err = cc.CarbonIntensityConfig.Validate()
	case EM:
		err = cc.ElectricityMapsConfig.Validate()
	default:
		err = fmt.Errorf("invalid carbon config: provider %s not recognized", cc.ProviderKey)
	}

	return
}

// Finalize sets the provider instances based on the user specified configuration.
func (cc *CarbonConfig) Finalize() (err error) {
	if cc == nil {
		return
	}

	if err = cc.Validate(); err != nil {
		return
	}

	var factoryFn func(*CarbonConfig) (CarbonScoreProvider, error)
	switch cc.ProviderKey {
	case AWS:
		factoryFn = newAWSProvider
	case GCP:
		factoryFn = newGCPProvider
	case AZ:
		factoryFn = newAzureProvider
	case CI:
		factoryFn = newCIProvider
	case EM:
		factoryFn = newEMProvider
	}

	if factoryFn == nil {
		return
	}

	var provider CarbonScoreProvider
	provider, err = factoryFn(cc)
	if err != nil {
		return
	}

	cc.Provider = &provider
	return
}

type AWSConfig struct {
	AccessKeyID     string `hcl:"access_key_id"`
	SecretAccessKey string `hcl:"secret_access_key"`
	SessionToken    string `hcl:"session_token"`
}

func (aws *AWSConfig) Validate() error {
	var mErr multierror.Error

	if aws == nil {
		return fmt.Errorf("invalid carbon config: AWS specified but not configured")
	}

	if aws.AccessKeyID == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: access_key_id required"))
	}

	if aws.SecretAccessKey == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: secret_acces_key required"))
	}

	if aws.SessionToken == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: session_token required"))
	}

	return mErr.ErrorOrNil()
}

func newAWSProvider(config *CarbonConfig) (CarbonScoreProvider, error) {
	return &awsProvider{
		config,
	}, nil
}

type awsProvider struct {
	carbonConfig *CarbonConfig
}

func (aws *awsProvider) GetCarbonIntensity(ctx context.Context) (float64, error) {
	return 0, nil
}

type gcpProvider struct {
	config *CarbonConfig
}

type GCPConfig struct {
	ServiceAccountKey string `hcl:"service_account_key"`
}

func (gcp *GCPConfig) Validate() error {
	if gcp == nil {
		return fmt.Errorf("invalid carbon config: GCP specified but not configured")
	}

	if gcp.ServiceAccountKey == "" {
		return fmt.Errorf("invalid carbon config: service_account_key required")
	}

	return nil
}

func newGCPProvider(config *CarbonConfig) (CarbonScoreProvider, error) {
	return &gcpProvider{
		config,
	}, nil
}

func (gcp *gcpProvider) GetCarbonIntensity(ctx context.Context) (float64, error) {
	return 0, nil
}

type azureProvider struct {
	config *CarbonConfig
}

type AzureConfig struct {
	ClientID     string `hcl:"client_id"`
	ClientSecret string `hcl:"client_secret"`
	TenantID     string `hcl:"tenant_id"`
}

func (az *AzureConfig) Validate() error {
	var mErr multierror.Error

	if az == nil {
		return fmt.Errorf("invalid carbon config: Azure specified but not configured")
	}

	if az.ClientID == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: client_id required"))
	}

	if az.ClientSecret == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: client_secret required"))
	}

	if az.TenantID == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: tenant_id required"))
	}

	return mErr.ErrorOrNil()
}

func newAzureProvider(config *CarbonConfig) (CarbonScoreProvider, error) {
	return &azureProvider{
		config,
	}, nil
}

func (az *azureProvider) GetCarbonIntensity(ctx context.Context) (float64, error) {
	return 0, nil
}

type ciProvider struct {
	config   *CarbonConfig
	provider grid.Provider
}

type CarbonIntensityConfig struct {
	APIUrl string `hcl:"api_url"`
}

func (ci *CarbonIntensityConfig) Validate() error {
	var mErr multierror.Error

	if ci == nil {
		return fmt.Errorf("invalid carbon config: Carbon Intensity specified but not configured")
	}

	if ci.APIUrl == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: api_url required"))
	}

	return mErr.ErrorOrNil()
}

func newCIProvider(config *CarbonConfig) (CarbonScoreProvider, error) {
	provider, err := ci.New(ci.WithAPIURL(config.CarbonIntensityConfig.APIUrl))
	if err != nil {
		return nil, err
	}

	return &ciProvider{
		config,
		provider,
	}, nil
}

func (ci *ciProvider) GetCarbonIntensity(ctx context.Context) (float64, error) {
	return ci.provider.GetCarbonIntensity(ctx, ci.config.Region)
}

type emProvider struct {
	config *ElectricityMapsConfig
}

type ElectricityMapsConfig struct {
	APIKey string `hcl:"api_key"`
	APIUrl string `hcl:"api_url"`
}

func (em *ElectricityMapsConfig) Validate() error {
	var mErr multierror.Error

	if em == nil {
		return fmt.Errorf("invalid carbon config: Electricity Maps specified but not configured")
	}

	if em.APIKey == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: api_key required"))
	}

	if em.APIUrl == "" {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid carbon config: api_url required"))
	}

	return mErr.ErrorOrNil()
}

func newEMProvider(config *CarbonConfig) (CarbonScoreProvider, error) {
	return &emProvider{
		config.ElectricityMapsConfig,
	}, nil
}

func (em *emProvider) GetCarbonIntensity(ctx context.Context) (float64, error) {
	return 0, nil
}
