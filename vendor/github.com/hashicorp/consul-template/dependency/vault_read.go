package dependency

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ Dependency = (*VaultReadQuery)(nil)
)

// VaultReadQuery is the dependency to Vault for a secret
type VaultReadQuery struct {
	stopCh chan struct{}

	path   string
	secret *Secret

	// vaultSecret is the actual Vault secret which we are renewing
	vaultSecret *api.Secret
}

// NewVaultReadQuery creates a new datacenter dependency.
func NewVaultReadQuery(s string) (*VaultReadQuery, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil, fmt.Errorf("vault.read: invalid format: %q", s)
	}

	return &VaultReadQuery{
		stopCh: make(chan struct{}, 1),
		path:   s,
	}, nil
}

// Fetch queries the Vault API
func (d *VaultReadQuery) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	select {
	case <-d.stopCh:
		return nil, nil, ErrStopped
	default:
	}

	opts = opts.Merge(&QueryOptions{})

	if d.secret != nil {
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
		} else {
			// The secret isn't renewable, probably the generic secret backend.
			dur := vaultRenewDuration(d.secret)
			if dur < opts.VaultGrace {
				log.Printf("[TRACE] %s: remaining lease %s is less than grace, skipping sleep", d, dur)
			} else {
				log.Printf("[TRACE] %s: secret is not renewable, sleeping for %s", d, dur)
				select {
				case <-time.After(dur):
					// The lease is almost expired, it's time to request a new one.
				case <-d.stopCh:
					return nil, nil, ErrStopped
				}
			}
		}
	}

	// We don't have a secret, or the prior renewal failed
	vaultSecret, err := d.readSecret(clients, opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	// Print any warnings
	printVaultWarnings(d, vaultSecret.Warnings)

	// Create the cloned secret which will be exposed to the template.
	d.vaultSecret = vaultSecret
	d.secret = transformSecret(vaultSecret)

	return respWithMetadata(d.secret)
}

// CanShare returns if this dependency is shareable.
func (d *VaultReadQuery) CanShare() bool {
	return false
}

// Stop halts the given dependency's fetch.
func (d *VaultReadQuery) Stop() {
	close(d.stopCh)
}

// String returns the human-friendly version of this dependency.
func (d *VaultReadQuery) String() string {
	return fmt.Sprintf("vault.read(%s)", d.path)
}

// Type returns the type of this dependency.
func (d *VaultReadQuery) Type() Type {
	return TypeVault
}

func (d *VaultReadQuery) readSecret(clients *ClientSet, opts *QueryOptions) (*api.Secret, error) {
	log.Printf("[TRACE] %s: GET %s", d, &url.URL{
		Path:     "/v1/" + d.path,
		RawQuery: opts.String(),
	})
	vaultSecret, err := clients.Vault().Logical().Read(d.path)
	if err != nil {
		return nil, errors.Wrap(err, d.String())
	}
	if vaultSecret == nil {
		return nil, fmt.Errorf("no secret exists at %s", d.path)
	}
	return vaultSecret, nil
}
