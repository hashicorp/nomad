package dependency

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

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

	// If this is not the first query and we have a lease duration, sleep until we
	// try to renew.
	if opts.WaitIndex != 0 && d.secret != nil && d.secret.LeaseDuration != 0 {
		dur := vaultRenewDuration(d.secret.LeaseDuration)

		log.Printf("[TRACE] %s: long polling for %s", d, dur)

		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(dur):
		}
	}

	// Attempt to renew the secret. If we do not have a secret or if that secret
	// is not renewable, we will attempt a (re-)read later.
	if d.secret != nil && d.secret.LeaseID != "" && d.secret.Renewable {
		log.Printf("[TRACE] %s: PUT %s", d, &url.URL{
			Path:     "/v1/sys/renew/" + d.secret.LeaseID,
			RawQuery: opts.String(),
		})

		renewal, err := clients.Vault().Sys().Renew(d.secret.LeaseID, 0)
		if err == nil {
			log.Printf("[TRACE] %s: successfully renewed %s", d, d.secret.LeaseID)

			// Print any warnings
			d.printWarnings(renewal.Warnings)

			secret := &Secret{
				RequestID:     renewal.RequestID,
				LeaseID:       renewal.LeaseID,
				LeaseDuration: d.secret.LeaseDuration,
				Renewable:     renewal.Renewable,
				Data:          d.secret.Data,
			}
			// For some older versions of Vault, the renewal did not include the
			// remaining lease duration, so just use the original lease duration,
			// because it's the best we can do.
			if renewal.LeaseDuration != 0 {
				secret.LeaseDuration = renewal.LeaseDuration
			}
			d.secret = secret

			// If the remaining time on the lease is less than or equal to our
			// configured grace period, generate a new credential now. This will help
			// minimize downtime, since Vault will revoke credentials immediately
			// when their maximum TTL expires.
			remaining := time.Duration(d.secret.LeaseDuration) * time.Second
			if remaining <= opts.VaultGrace {
				log.Printf("[DEBUG] %s: remaining lease (%s) < grace (%s), acquiring new",
					d, remaining, opts.VaultGrace)
				return d.readSecret(clients, opts)
			}

			return respWithMetadata(secret)
		}

		// The renewal failed for some reason.
		log.Printf("[WARN] %s: failed to renew %s: %s", d, d.secret.LeaseID, err)
	}

	// If we got this far, we either didn't have a secret to renew, the secret was
	// not renewable, or the renewal failed, so attempt a fresh read.
	return d.readSecret(clients, opts)
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

func (d *VaultReadQuery) printWarnings(warnings []string) {
	for _, w := range warnings {
		log.Printf("[WARN] %s: %s", d, w)
	}
}

func (d *VaultReadQuery) readSecret(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	log.Printf("[TRACE] %s: GET %s", d, &url.URL{
		Path:     "/v1/" + d.path,
		RawQuery: opts.String(),
	})
	vaultSecret, err := clients.Vault().Logical().Read(d.path)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	// The secret could be nil if it does not exist.
	if vaultSecret == nil {
		return nil, nil, fmt.Errorf("%s: no secret exists at %s", d, d.path)
	}

	// Print any warnings.
	d.printWarnings(vaultSecret.Warnings)

	// Create our cloned secret.
	secret := &Secret{
		LeaseID:       vaultSecret.LeaseID,
		LeaseDuration: leaseDurationOrDefault(vaultSecret.LeaseDuration),
		Renewable:     vaultSecret.Renewable,
		Data:          vaultSecret.Data,
	}
	d.secret = secret

	return respWithMetadata(secret)
}
