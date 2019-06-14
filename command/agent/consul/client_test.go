package consul

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConsul_RemoveStalePendingRemovals(t *testing.T) {
	now := time.Now()
	recent := now.Add(-5 * time.Second)
	c := &ServiceClient{
		servicesPendingRemoval: map[string]time.Time{
			"service_superold": now.Add(-24 * time.Hour),
			"service_recent":   recent,
		},
		checksPendingRemoval: map[string]time.Time{
			"checks_superold": now.Add(-24 * time.Hour),
			"checks_recent":   recent,
		},
	}

	c.removeStalePendingRemovals()

	require.Equal(t, map[string]time.Time{"service_recent": recent}, c.servicesPendingRemoval)
	require.Equal(t, map[string]time.Time{"checks_recent": recent}, c.checksPendingRemoval)
}
