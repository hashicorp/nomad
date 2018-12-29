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

	"github.com/hashicorp/vault/api"
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

	// vaultSecret is the actual Vault secret which we are renewing
	vaultSecret *api.Secret
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
			log.Printf("[TRACE] %s: secret is not renewable, sleeping for %s", d, dur)
			select {
			case <-time.After(dur):
				// The lease is almost expired, it's time to request a new one.
			case <-d.stopCh:
				return nil, nil, ErrStopped
			}
		}
	}

	// We don't have a secret, or the prior renewal failed
	vaultSecret, err := d.writeSecret(clients, opts)
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

func (d *VaultWriteQuery) writeSecret(clients *ClientSet, opts *QueryOptions) (*api.Secret, error) {
	log.Printf("[TRACE] %s: PUT %s", d, &url.URL{
		Path:     "/v1/" + d.path,
		RawQuery: opts.String(),
	})

	vaultSecret, err := clients.Vault().Logical().Write(d.path, d.data)
	if err != nil {
		return nil, errors.Wrap(err, d.String())
	}
	if vaultSecret == nil {
		return nil, fmt.Errorf("no secret exists at %s", d.path)
	}

	return vaultSecret, nil
}
