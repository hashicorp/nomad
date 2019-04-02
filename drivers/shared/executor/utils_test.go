package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInsideBase(t *testing.T) {

	cases := []struct {
		expected bool
		base     string
		path     string
	}{
		{false, "/usr/bin", "/"},
		{false, "/usr/bin", "/opt/usr"},
		{false, "/usr/bin", "/usr"},
		{true, "/usr/bin", "/usr/bin"},
		{true, "/usr/bin", "/usr/bin/"},
		{true, "/usr/bin", "/usr/bin/child"},
		{true, "/usr/bin", "/usr/bin/child/grandchild"},
	}

	for _, c := range cases {
		if c.expected {
			assert.Truef(t, insideBase(c.base, c.path), "path %q is expected to be child of %q", c.path, c.base)
		} else {
			assert.Falsef(t, insideBase(c.base, c.path), "path %q is not expected to be child of %q", c.path, c.base)
		}
	}
}
