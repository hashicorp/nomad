//go:build unix

package users

import (
	"fmt"
	"os/user"
	"strconv"
)

var (
	// nobody is a cached copy of the nobody user, which is going to be looked-up
	// frequently and is unlikely to be modified on the underlying system.
	nobody user.User

	// nobodyUID is a cached copy of the int value of the nobody user's UID.
	nobodyUID uint32

	// nobodyGID int is a cached copy of the int value of the nobody users's GID.
	nobodyGID uint32
)

// Nobody returns User data for the "nobody" user on the system, bypassing the
// locking / file read / NSS lookup.
func Nobody() user.User {
	return nobody
}

// NobodyIDs returns the integer UID and GID of the nobody user.
func NobodyIDs() (uint32, uint32) {
	return nobodyUID, nobodyGID
}

func init() {
	u, err := Lookup("nobody")
	if err != nil {
		panic(fmt.Sprintf("failed to lookup nobody user: %v", err))
	}
	nobody = *u

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		panic(fmt.Sprintf("failed to parse nobody UID: %v", err))
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		panic(fmt.Sprintf("failed to parse nobody GID: %v", err))
	}

	nobodyUID, nobodyGID = uint32(uid), uint32(gid)
}
