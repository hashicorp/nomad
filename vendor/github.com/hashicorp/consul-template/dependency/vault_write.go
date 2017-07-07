package dependency

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var (
	// Ensure implements
	_ Dependency = (*VaultWriteQuery)(nil)
)

// VaultWriteQuery is the dependency to Vault for a secret
type VaultWriteQuery struct {
	stopCh chan struct{}

	path     string
	data     map[string]interface{}
	dataHash string
	secret   *Secret
}

// NewVaultWriteQuery creates a new datacenter dependency.
func NewVaultWriteQuery(s string, d map[string]interface{}) (*VaultWriteQuery, error) {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "/")
	if s == "" {
		return nil, fmt.Errorf("vault.write: invalid format: %q", s)
	}

	return &VaultWriteQuery{
		stopCh:   make(chan struct{}, 1),
		path:     s,
		data:     d,
		dataHash: sha1Map(d),
	}, nil
}

// Fetch queries the Vault API
func (d *VaultWriteQuery) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
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
	// is not renewable, we will attempt a (re-)write later.
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
				RequestID: renewal.RequestID,
				LeaseID:   renewal.LeaseID,
				Renewable: renewal.Renewable,
				Data:      d.secret.Data,
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
				return d.writeSecret(clients, opts)
			}

			return respWithMetadata(secret)
		}

		// The renewal failed for some reason.
		log.Printf("[WARN] %s: failed to renew %s: %s", d, d.secret.LeaseID, err)
	}

	// If we got this far, we either didn't have a secret to renew, the secret was
	// not renewable, or the renewal failed, so attempt a fresh write.
	return d.writeSecret(clients, opts)
}

// CanShare returns if this dependency is shareable.
func (d *VaultWriteQuery) CanShare() bool {
	return false
}

// Stop halts the given dependency's fetch.
func (d *VaultWriteQuery) Stop() {
	close(d.stopCh)
}

// String returns the human-friendly version of this dependency.
func (d *VaultWriteQuery) String() string {
	return fmt.Sprintf("vault.write(%s -> %s)", d.path, d.dataHash)
}

// Type returns the type of this dependency.
func (d *VaultWriteQuery) Type() Type {
	return TypeVault
}

// sha1Map returns the sha1 hash of the data in the map. The reason this data is
// hashed is because it appears in the output and could contain sensitive
// information.
func sha1Map(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha1.New()
	for _, k := range keys {
		io.WriteString(h, fmt.Sprintf("%s=%q", k, m[k]))
	}

	return fmt.Sprintf("%.4x", h.Sum(nil))
}

func (d *VaultWriteQuery) printWarnings(warnings []string) {
	for _, w := range warnings {
		log.Printf("[WARN] %s: %s", d, w)
	}
}

func (d *VaultWriteQuery) writeSecret(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	log.Printf("[TRACE] %s: PUT %s", d, &url.URL{
		Path:     "/v1/" + d.path,
		RawQuery: opts.String(),
	})

	vaultSecret, err := clients.Vault().Logical().Write(d.path, d.data)
	if err != nil {
		return nil, nil, errors.Wrap(err, d.String())
	}

	// The secret could be nil if it does not exist.
	if vaultSecret == nil {
		return nil, nil, fmt.Errorf("%s: no secret exists at %s", d, d.path)
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
