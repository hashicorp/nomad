package userlookup

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"strings"
)

// Lookup checks if the given username or uid is present in /etc/passwd
// and returns the user struct.
// If the username is not found, an error is returned.
// Credit to @creak, https://github.com/docker/docker/pull/1096
func Lookup(uid string) (*user.User, error) {
	file, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(file), "\n") {
		data := strings.Split(line, ":")
		if len(data) > 5 && (data[0] == uid || data[2] == uid) {
			return &user.User{
				Uid:      data[2],
				Gid:      data[3],
				Username: data[0],
				Name:     data[4],
				HomeDir:  data[5],
			}, nil
		}
	}
	return nil, fmt.Errorf("User not found in /etc/passwd")
}
