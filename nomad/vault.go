package nomad

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	vapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
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
	auth   *vapi.TokenAuth
	logger *log.Logger

	// running returns whether the renewal goroutine is running
	running    bool
	shutdownCh chan struct{}
	l          sync.Mutex

	// enabled indicates whether the vaultClient is enabled. If it is not the
	// token lookup and create methods will return errors.
	enabled bool

	// tokenRole is the role in which child tokens will be created from.
	tokenRole string

	// childTTL is the TTL for child tokens.
	childTTL string

	// leaseDuration is the lease duration of our token in seconds
	leaseDuration int

	// lastRenewed is the time the token was last renewed
	lastRenewed time.Time
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
	if client.Token() == "" {
		client.SetToken(c.PeriodicToken)
	}

	v.client = client
	v.auth = client.Auth().Token()

	// Validate we have the required fields. This is done after we create the
	// client since these fields can be read from environment variables
	if c.TokenRoleName == "" {
		return nil, errors.New("Vault token role name must be set in config")
	} else if client.Token() == "" {
		return nil, errors.New("Vault periodic token must be set in config or in $VAULT_TOKEN")
	} else if apiConf.Address == "" {
		return nil, errors.New("Vault address must be set in config or in $VAULT_ADDR")
	}

	// Retrieve our token, validate it and parse the lease duration
	leaseDuration, err := v.parseSelfToken()
	if err != nil {
		return nil, err
	}
	v.leaseDuration = leaseDuration

	v.logger.Printf("[DEBUG] vault: token lease duration is %v",
		time.Duration(v.leaseDuration)*time.Second)

	// Prepare and launch the token renewal goroutine
	v.shutdownCh = make(chan struct{})
	v.running = true
	go v.run()

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

func (v *vaultClient) parseSelfToken() (int, error) {
	// Get the initial lease duration
	auth := v.client.Auth().Token()
	self, err := auth.LookupSelf()
	if err != nil {
		return 0, fmt.Errorf("failed to lookup Vault periodic token: %v", err)
	}

	// Read and parse the fields
	var data struct {
		CreationTTL int  `mapstructure:"creation_ttl"`
		TTL         int  `mapstructure:"ttl"`
		Renewable   bool `mapstructure:"renewable"`
	}
	if err := mapstructure.WeakDecode(self.Data, &data); err != nil {
		return 0, fmt.Errorf("failed to parse Vault token's data block: %v", err)
	}

	if !data.Renewable {
		return 0, fmt.Errorf("Vault token is not renewable")
	}

	if data.CreationTTL == 0 {
		return 0, fmt.Errorf("invalid lease duration of zero")
	}

	if data.TTL == 0 {
		return 0, fmt.Errorf("token TTL is zero")
	}

	return data.CreationTTL, nil
}

// run runs the renew loop
func (v *vaultClient) run() {
	// Create the renewal timer and set initial duration to zero so it fires
	// immediately
	authRenewTimer := time.NewTimer(0)

	// Backoff is to reduce the rate we try to renew with Vault under error
	// situations
	backoff := 0.0

	for {
		select {
		case <-v.shutdownCh:
			return
		case <-authRenewTimer.C:
			err := v.renew()
			currentExpiration := v.lastRenewed.Add(time.Duration(v.leaseDuration) * time.Second)

			// Successfully renewed
			if err == nil {
				// If we take the expiration (lastRenewed + auth duration) and
				// subtract the current time, we get a duration until expiry.
				// Set the timer to poke us after half of that time is up.
				durationUntilRenew := currentExpiration.Sub(time.Now()) / 2

				v.logger.Printf("[INFO] vault: renewing token in %v", durationUntilRenew)
				authRenewTimer.Reset(durationUntilRenew)

				// Reset any backoff
				backoff = 0
				break
			}

			// Back off, increasing the amount of backoff each time. There are some rules:
			//
			// * If we have an existing authentication that is going to expire,
			// never back off more than half of the amount of time remaining
			// until expiration
			// * Never back off more than 30 seconds multiplied by a random
			// value between 1 and 2
			// * Use randomness so that many clients won't keep hitting Vault
			// at the same time

			// Set base values and add some backoff

			v.logger.Printf("[DEBUG] vault: got error or bad auth, so backing off: %v", err)
			switch {
			case backoff < 5:
				backoff = 5
			case backoff >= 24:
				backoff = 30
			default:
				backoff = backoff * 1.25
			}

			// Add randomness
			backoff = backoff * (1.0 + rand.Float64())

			maxBackoff := currentExpiration.Sub(time.Now()) / 2
			if maxBackoff < 0 {
				// We have failed to renew the token past its expiration. Stop
				// renewing with Vault.
				v.l.Lock()
				defer v.l.Unlock()
				v.logger.Printf("[ERR] vault: failed to renew Vault token before lease expiration. Renew loop exiting")
				if v.running {
					v.running = false
					close(v.shutdownCh)
				}

				return

			} else if backoff > maxBackoff.Seconds() {
				backoff = maxBackoff.Seconds()
			}

			durationUntilRetry := time.Duration(backoff) * time.Second
			v.logger.Printf("[INFO] vault: backing off for %v", durationUntilRetry)

			authRenewTimer.Reset(durationUntilRetry)
		}
	}
}

func (v *vaultClient) renew() error {
	// Attempt to renew the token
	secret, err := v.auth.RenewSelf(v.leaseDuration)
	if err != nil {
		return err
	}

	auth := secret.Auth
	if auth == nil {
		return fmt.Errorf("renewal successful but not auth information returned")
	} else if auth.LeaseDuration == 0 {
		return fmt.Errorf("renewal successful but no lease duration returned")
	}

	v.lastRenewed = time.Now()
	v.logger.Printf("[DEBUG] vault: succesfully renewed server token")
	return nil
}

// Stop stops token renewal.
func (v *vaultClient) Stop() {
	// Nothing to do
	if !v.enabled {
		return
	}

	v.l.Lock()
	defer v.l.Unlock()
	if !v.running {
		return
	}

	close(v.shutdownCh)
	v.running = false
}

func (v *vaultClient) CreateToken(a *structs.Allocation, task string) (*vapi.Secret, error) {
	return nil, nil
}

func (v *vaultClient) LookupToken(token string) (*vapi.Secret, error) {
	return nil, nil
}
