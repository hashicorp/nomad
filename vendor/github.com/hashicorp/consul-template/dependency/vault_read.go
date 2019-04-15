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

	rawPath     string
	queryValues url.Values
	secret      *Secret
	isKVv2      *bool
	secretPath  string

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

	secretURL, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	return &VaultReadQuery{
		stopCh:      make(chan struct{}, 1),
		rawPath:     secretURL.Path,
		queryValues: secretURL.Query(),
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
	return fmt.Sprintf("vault.read(%s)", d.rawPath)
}

// Type returns the type of this dependency.
func (d *VaultReadQuery) Type() Type {
	return TypeVault
}

func (d *VaultReadQuery) readSecret(clients *ClientSet, opts *QueryOptions) (*api.Secret, error) {
	vaultClient := clients.Vault()

	// Check whether this secret refers to a KV v2 entry if we haven't yet.
	if d.isKVv2 == nil {
		mountPath, isKVv2, err := isKVv2(vaultClient, d.rawPath)
		if err != nil {
			return nil, errors.Wrap(err, d.String())
		}

		if isKVv2 {
			d.secretPath = addPrefixToVKVPath(d.rawPath, mountPath, "data")
		} else {
			d.secretPath = d.rawPath
		}
		d.isKVv2 = &isKVv2
	}

	queryString := d.queryValues.Encode()
	log.Printf("[TRACE] %s: GET %s", d, &url.URL{
		Path:     "/v1/" + d.secretPath,
		RawQuery: queryString,
	})
	vaultSecret, err := vaultClient.Logical().ReadWithData(d.secretPath, d.queryValues)
	if err != nil {
		return nil, errors.Wrap(err, d.String())
	}
	if vaultSecret == nil {
		return nil, fmt.Errorf("no secret exists at %s", d.secretPath)
	}
	return vaultSecret, nil
}
