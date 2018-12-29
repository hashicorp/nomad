package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCgroupPermission(t *testing.T) {
	positiveCases := []string{
		"r",
		"rw",
		"rwm",
		"mr",
		"mrw",
		"",
	}

	for _, c := range positiveCases {
		t.Run("positive case: "+c, func(t *testing.T) {
			require.True(t, validateCgroupPermission(c))
		})
	}

	negativeCases := []string{
		"q",
		"asdf",
		"rq",
	}

	for _, c := range negativeCases {
		t.Run("negative case: "+c, func(t *testing.T) {
			require.False(t, validateCgroupPermission(c))
		})
	}

}
