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
		path:   s,
		stopCh: make(chan struct{}, 1),
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
		dur := time.Duration(d.secret.LeaseDuration/2.0) * time.Second
		if dur == 0 {
			dur = time.Duration(VaultDefaultLeaseDuration)
		}

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

			secret := &Secret{
				RequestID:     renewal.RequestID,
				LeaseID:       renewal.LeaseID,
				LeaseDuration: d.secret.LeaseDuration,
				Renewable:     renewal.Renewable,
				Data:          d.secret.Data,
			}
			d.secret = secret

			return respWithMetadata(secret)
		}

		// The renewal failed for some reason.
		log.Printf("[WARN] %s: failed to renew %s: %s", d, d.secret.LeaseID, err)
	}

	// If we got this far, we either didn't have a secret to renew, the secret was
	// not renewable, or the renewal failed, so attempt a fresh read.
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
		log.Printf("[WARN] %s: returned nil (does the secret exist?)", d)
		return respWithMetadata(nil)
	}

	// Print any warnings.
	for _, w := range vaultSecret.Warnings {
		log.Printf("[WARN] %s: %s", d, w)
	}

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
