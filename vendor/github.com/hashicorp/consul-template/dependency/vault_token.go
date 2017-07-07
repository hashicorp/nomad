package dependency

import (
	"log"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ Dependency = (*VaultTokenQuery)(nil)
)

// VaultTokenQuery is the dependency to Vault for a secret
type VaultTokenQuery struct {
	stopCh chan struct{}

	leaseID       string
	leaseDuration int
}

// NewVaultTokenQuery creates a new dependency.
func NewVaultTokenQuery() (*VaultTokenQuery, error) {
	return &VaultTokenQuery{
		stopCh: make(chan struct{}, 1),
	}, nil
}

// Fetch queries the Vault API
func (d *VaultTokenQuery) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts = opts.Merge(&QueryOptions{})

	log.Printf("[TRACE] %s: GET %s", d, &url.URL{
		Path:     "/v1/auth/token/renew-self",
		RawQuery: opts.String(),
	})

	// If this is not the first query and we have a lease duration, sleep until we
	// try to renew.
	if opts.WaitIndex != 0 && d.leaseDuration != 0 {
		dur := vaultRenewDuration(d.leaseDuration)

		log.Printf("[TRACE] %s: long polling for %s", d, dur)

		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(dur):
		}
	}

	token, err := clients.Vault().Auth().Token().RenewSelf(0)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	// Create our cloned secret
	secret := &Secret{
		LeaseID:       token.LeaseID,
		LeaseDuration: token.Auth.LeaseDuration,
		Renewable:     token.Auth.Renewable,
		Data:          token.Data,
	}

	d.leaseID = secret.LeaseID
	d.leaseDuration = secret.LeaseDuration

	log.Printf("[DEBUG] %s: renewed token", d)

	return respWithMetadata(secret)
}

// CanShare returns if this dependency is shareable.
func (d *VaultTokenQuery) CanShare() bool {
	return false
}

// Stop halts the dependency's fetch function.
func (d *VaultTokenQuery) Stop() {
	close(d.stopCh)
}

// String returns the human-friendly version of this dependency.
func (d *VaultTokenQuery) String() string {
	return "vault.token"
}

// Type returns the type of this dependency.
func (d *VaultTokenQuery) Type() Type {
	return TypeVault
}
