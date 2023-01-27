package users

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// lock is used to serialize all user lookup at the process level, because
// some NSS implementations are not concurrency safe
var lock sync.Mutex

// Lookup username while holding a global process lock.
func Lookup(username string) (*user.User, error) {
	lock.Lock()
	defer lock.Unlock()
	return user.Lookup(username)
}

// LookupGroupId while holding a global process lock.
func LookupGroupId(gid string) (*user.Group, error) {
	lock.Lock()
	defer lock.Unlock()
	return user.LookupGroupId(gid)
}

// Current returns the current user, acquired while holding a global process
// lock.
func Current() (*user.User, error) {
	lock.Lock()
	defer lock.Unlock()
	return user.Current()
}

// WriteFileFor is like os.WriteFile except if possible it chowns the file to
// the specified user (possibly from Task.User) and sets the permissions to
// 0o600.
//
// If chowning fails (either due to OS or Nomad being unprivileged), the file
// will be left world readable (0o666).
//
// On failure a multierror with both the original and fallback errors will be
// returned.
func WriteFileFor(path string, contents []byte, username string) error {
	// Don't even bother trying to chown to an empty username
	var origErr error
	if username != "" {
		origErr := writeFileFor(path, contents, username)
		if origErr == nil {
			// Success!
			return nil
		}
	}

	// Fallback to world readable
	if err := os.WriteFile(path, contents, 0o666); err != nil {
		if origErr != nil {
			// Return both errors
			return &multierror.Error{
				Errors: []error{origErr, err},
			}
		} else {
			return err
		}
	}

	return nil
}

func writeFileFor(path string, contents []byte, username string) error {
	user, err := Lookup(username)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		return fmt.Errorf("error parsing uid: %w", err)
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return err
	}

	if err := os.Chown(path, uid, -1); err != nil {
		// Delete the file so that the fallback method properly resets
		// permissions.
		_ = os.Remove(path)
		return err
	}

	return nil
}
