//go:build unix

package users

import (
	"fmt"
	"os/user"
)

// nobody is a cached copy of the nobody user, which is going to be looked-up
// frequently and is unlikely to be modified on the underlying system.
var nobody user.User

// Nobody returns User data for the "nobody" user on the system, bypassing the
// locking / file read / NSS lookup.
func Nobody() user.User {
	return nobody
}

func init() {
	u, err := Lookup("nobody")
	if err != nil {
		panic(fmt.Sprintf("failed to lookup nobody user: %v", err))
	}
	nobody = *u
}
