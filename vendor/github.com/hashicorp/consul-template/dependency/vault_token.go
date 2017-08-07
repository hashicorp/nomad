package dependency

import (
	"log"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ Dependency = (*VaultTokenQuery)(nil)
)

// VaultTokenQuery is the dependency to Vault for a secret
type VaultTokenQuery struct {
	stopCh      chan struct{}
	secret      *Secret
	vaultSecret *api.Secret
}

// NewVaultTokenQuery creates a new dependency.
func NewVaultTokenQuery(token string) (*VaultTokenQuery, error) {
	vaultSecret := &api.Secret{
		Auth: &api.SecretAuth{
			ClientToken:   token,
			Renewable:     true,
			LeaseDuration: 1,
		},
	}
	return &VaultTokenQuery{
		stopCh:      make(chan struct{}, 1),
		vaultSecret: vaultSecret,
		secret:      transformSecret(vaultSecret),
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

	if vaultSecretRenewable(d.secret) {
		log.Printf("[TRACE] %s: starting renewer", d)

		renewer, err := clients.Vault().NewRenewer(&api.RenewerInput{
			Grace:  opts.VaultGrace,
			Secret: d.vaultSecret,
		})
		if err != nil {
			return nil, nil, errors.Wrap(err, d.String())
		}
		go renewer.Renew()
		defer renewer.Stop()

	RENEW:
		for {
			select {
			case err := <-renewer.DoneCh():
				if err != nil {
					log.Printf("[WARN] %s: failed to renew: %s", d, err)
				}
				log.Printf("[WARN] %s: renewer returned (maybe the lease expired)", d)
				break RENEW
			case renewal := <-renewer.RenewCh():
				log.Printf("[TRACE] %s: successfully renewed", d)
				printVaultWarnings(d, renewal.Secret.Warnings)
				updateSecret(d.secret, renewal.Secret)
			case <-d.stopCh:
				return nil, nil, ErrStopped
			}
		}
	}

	// The secret isn't renewable, probably the generic secret backend.
	dur := vaultRenewDuration(d.secret)
	if dur < opts.VaultGrace {
		log.Printf("[TRACE] %s: remaining lease %s is less than grace, skipping sleep", d, dur)
	} else {
		log.Printf("[TRACE] %s: token is not renewable, sleeping for %s", d, dur)
		select {
		case <-time.After(dur):
			// The lease is almost expired, it's time to request a new one.
		case <-d.stopCh:
			return nil, nil, ErrStopped
		}
	}

	return nil, nil, ErrLeaseExpired
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
