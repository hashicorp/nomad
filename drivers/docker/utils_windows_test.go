// +build windows

package docker

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	cases := []struct {
		base     string
		target   string
		expected string
	}{
		{"/tmp/alloc/task", ".", "/tmp/alloc/task"},
		{"/tmp/alloc/task", "..", "/tmp/alloc"},

		{"/tmp/alloc/task", "d1/d2", "/tmp/alloc/task/d1/d2"},
		{"/tmp/alloc/task", "../d1/d2", "/tmp/alloc/d1/d2"},
		{"/tmp/alloc/task", "../../d1/d2", "/tmp/d1/d2"},

		{"/tmp/alloc/task", "c:/home/user", "c:/home/user"},
		{"/tmp/alloc/task", "c:/home/user/..", "c:/home"},
	}

	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			require.Equal(t, c.expected, filepath.ToSlash(expandPath(c.base, c.target)))
		})
	}
}
