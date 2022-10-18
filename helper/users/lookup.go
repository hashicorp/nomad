package users

import (
	"os/user"
	"sync"
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
