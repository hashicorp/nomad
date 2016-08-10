package nomad

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
)

const (
	// vaultTokenCreateTTL is the duration the wrapped token for the client is
	// valid for. The units are in seconds.
	vaultTokenCreateTTL = "60s"

	// minimumTokenTTL is the minimum Token TTL allowed for child tokens.
	minimumTokenTTL = 5 * time.Minute
)

// VaultClient is the Servers interface for interfacing with Vault
type VaultClient interface {
	// CreateToken takes an allocation and task and returns an appropriate Vault
	// Secret
	CreateToken(a *structs.Allocation, task string) (*vapi.Secret, error)

	// LookupToken takes a token string and returns its capabilities.
	LookupToken(token string) (*vapi.Secret, error)

	// Stop is used to stop token renewal.
	Stop()
}

// vaultClient is the Servers implementation of the VaultClient interface. The
// client renews the PeriodicToken given in the Vault configuration and provides
// the Server with the ability to create child tokens and lookup the permissions
// of tokens.
type vaultClient struct {
	// client is the Vault API client
	client *vapi.Client
	logger *log.Logger

	// enabled indicates whether the vaultClient is enabled. If it is not the
	// token lookup and create methods will return errors.
	enabled bool

	// tokenRole is the role in which child tokens will be created from.
	tokenRole string

	// childTTL is the TTL for child tokens.
	childTTL string
}

func NewVaultClient(c *config.VaultConfig, logger *log.Logger) (*vaultClient, error) {
	if c == nil {
		return nil, fmt.Errorf("must pass valid VaultConfig")
	}

	if logger == nil {
		return nil, fmt.Errorf("must pass valid logger")
	}

	v := &vaultClient{
		enabled:   c.Enabled,
		tokenRole: c.TokenRoleName,
		logger:    logger,
	}

	// If vault is not enabled do not configure an API client or start any token
	// renewal.
	if !v.enabled {
		return v, nil
	}

	// Validate we have the required fields
	if c.TokenRoleName == "" {
		return nil, errors.New("Vault token role name must be set")
	} else if c.PeriodicToken == "" {
		return nil, errors.New("Vault periodic token must be set")
	} else if c.Addr == "" {
		return nil, errors.New("Vault address must be set")
	}

	// Parse the TTL if it is set
	if c.ChildTokenTTL != "" {
		d, err := time.ParseDuration(c.ChildTokenTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ChildTokenTTL %q: %v", c.ChildTokenTTL, err)
		}

		// TODO this should be a config validation problem as well
		if d.Nanoseconds() < minimumTokenTTL.Nanoseconds() {
			return nil, fmt.Errorf("ChildTokenTTL is less than minimum allowed of %v", minimumTokenTTL)
		}

		v.childTTL = c.ChildTokenTTL
	}

	// Get the Vault API configuration
	apiConf, err := c.ApiConfig(true)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Vault API config: %v", err)
	}

	// Create the Vault API client
	client, err := vapi.NewClient(apiConf)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Vault API client: %v", err)
	}

	// Set the wrapping function such that token creation is wrapped
	client.SetWrappingLookupFunc(v.getWrappingFn())

	// Set the token and store the client
	client.SetToken(c.PeriodicToken)
	v.client = client
	return v, nil
}

// getWrappingFn returns an appropriate wrapping function for Nomad Servers
func (v *vaultClient) getWrappingFn() func(operation, path string) string {
	createPath := fmt.Sprintf("auth/token/create/%s", v.tokenRole)
	return func(operation, path string) string {
		// Only wrap the token create operation
		if operation != "POST" || path != createPath {
			return ""
		}

		return vaultTokenCreateTTL
	}
}

func (v *vaultClient) Stop() {
}

func (v *vaultClient) CreateToken(a *structs.Allocation, task string) (*vapi.Secret, error) {
	return nil, nil
}

func (v *vaultClient) LookupToken(token string) (*vapi.Secret, error) {
	return nil, nil
}
