package users

import (
	"os/user"
	"sync"
)

// lock is used to serialize all user lookup at the process level, because
// some NSS implementations are not concurrency safe
var lock sync.Mutex

// nobody is a cached copy of the nobody user, which is going to be looked-up
// frequently and is unlikely to be modified on the underlying system.
var nobody *user.User

// Nobody returns User data for the "nobody" user on the system, bypassing the
// locking / file read / NSS lookup.
func Nobody() (*user.User, error) {
	lock.Lock()
	defer lock.Unlock()
	if nobody != nil {
		return nobody, nil
	}
	u, err := user.Lookup("nobody")
	nobody = u
	return u, err
}

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
