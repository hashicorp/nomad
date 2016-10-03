package dependency

import (
	"log"
	"sync"
	"time"
)

// VaultToken is the dependency to Vault for a secret
type VaultToken struct {
	sync.Mutex

	leaseID       string
	leaseDuration int

	stopped bool
	stopCh  chan struct{}
}

// Fetch queries the Vault API
func (d *VaultToken) Fetch(clients *ClientSet, opts *QueryOptions) (interface{}, *ResponseMetadata, error) {
	d.Lock()
	if d.stopped {
		defer d.Unlock()
		return nil, nil, ErrStopped
	}
	d.Unlock()

	if opts == nil {
		opts = &QueryOptions{}
	}

	log.Printf("[DEBUG] (%s) renewing vault token", d.Display())

	// If this is not the first query and we have a lease duration, sleep until we
	// try to renew.
	if opts.WaitIndex != 0 && d.leaseDuration != 0 {
		duration := time.Duration(d.leaseDuration/2.0) * time.Second

		if duration < 1*time.Second {
			log.Printf("[DEBUG] (%s) increasing sleep to 1s (was %q)",
				d.Display(), duration)
			duration = 1 * time.Second
		}

		log.Printf("[DEBUG] (%s) sleeping for %q", d.Display(), duration)
		select {
		case <-d.stopCh:
			return nil, nil, ErrStopped
		case <-time.After(duration):
		}
	}

	// Grab the vault client
	vault, err := clients.Vault()
	if err != nil {
		return nil, nil, ErrWithExitf("vault_token: %s", err)
	}

	token, err := vault.Auth().Token().RenewSelf(0)
	if err != nil {
		return nil, nil, ErrWithExitf("error renewing vault token: %s", err)
	}

	// Create our cloned secret
	secret := &Secret{
		LeaseID:       token.LeaseID,
		LeaseDuration: token.Auth.LeaseDuration,
		Renewable:     token.Auth.Renewable,
		Data:          token.Data,
	}

	leaseDuration := token.Auth.LeaseDuration
	if leaseDuration == 0 {
		log.Printf("[WARN] (%s) lease duration is 0, setting to 5s", d.Display())
		leaseDuration = 5
	}

	d.Lock()
	d.leaseID = secret.LeaseID
	d.leaseDuration = leaseDuration
	d.Unlock()

	log.Printf("[DEBUG] (%s) successfully renewed token", d.Display())

	return respWithMetadata(secret)
}

// CanShare returns if this dependency is shareable.
func (d *VaultToken) CanShare() bool {
	return false
}

// HashCode returns the hash code for this dependency.
func (d *VaultToken) HashCode() string {
	return "VaultToken"
}

// Display returns a string that should be displayed to the user in output (for
// example).
func (d *VaultToken) Display() string {
	return "vault_token"
}

// Stop halts the dependency's fetch function.
func (d *VaultToken) Stop() {
	d.Lock()
	defer d.Unlock()

	if !d.stopped {
		close(d.stopCh)
		d.stopped = true
	}
}

// ParseVaultToken creates a new VaultToken dependency.
func ParseVaultToken() (*VaultToken, error) {
	return &VaultToken{stopCh: make(chan struct{})}, nil
}
