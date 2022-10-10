//go:build unix

package users

import (
	"fmt"
)

func init() {
	u, err := Lookup("nobody")
	if err != nil {
		panic(fmt.Sprintf("failed to lookup nobody user: %v", err))
	}
	nobody = *u
}
