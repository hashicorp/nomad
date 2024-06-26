package types

import (
	"fmt"
	"testing"
)

func TestSmoke(t *testing.T) {
	j := &Job{}
	fmt.Printf("%#v\n", j)
}
