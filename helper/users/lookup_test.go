//go:build linux

package users

import (
	"errors"
	"os/user"
	"testing"

	"github.com/shoenig/test/must"
)

func TestLookup(t *testing.T) {
	cases := []struct {
		username string

		expErr  error
		expUser *user.User
	}{
		{username: "nobody", expUser: &user.User{Username: "nobody", Uid: "65534", Gid: "65534", Name: "nobody", HomeDir: "/nonexistent"}}, // ubuntu
		{username: "root", expUser: &user.User{Username: "root", Uid: "0", Gid: "0", Name: "root", HomeDir: "/root"}},
		{username: "doesnotexist", expErr: errors.New("user: unknown user doesnotexist")},
	}

	for _, tc := range cases {
		t.Run(tc.username, func(t *testing.T) {
			u, err := Lookup(tc.username)
			if tc.expErr != nil {
				must.EqError(t, tc.expErr, err.Error())
			} else {
				must.Eq(t, tc.expUser, u)
			}
		})
	}
}

func TestLookup_NobodyIDs(t *testing.T) {
	uid, gid := NobodyIDs()
	must.Eq(t, 65534, uid) // ubuntu
	must.Eq(t, 65534, gid) // ubuntu
}
