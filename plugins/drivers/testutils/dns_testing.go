package testutils

import (
	"strings"
	"testing"

	dresolvconf "github.com/docker/libnetwork/resolvconf"
	dtypes "github.com/docker/libnetwork/types"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/stretchr/testify/require"
)

// TestTaskDNSConfig asserts that a task is running with the given DNSConfig
func TestTaskDNSConfig(t *testing.T, driver *DriverHarness, taskID string, dns *drivers.DNSConfig) {
	t.Run("dns_config", func(t *testing.T) {
		caps, err := driver.Capabilities()
		require.NoError(t, err)

		isolated := (caps.FSIsolation != drivers.FSIsolationNone)
		if !isolated {
			t.Skip("dns config not supported on non isolated drivers")
		}

		// write to a file and check it presence in host
		r := execTask(t, driver, taskID, `cat /etc/resolv.conf`,
			false, "")
		require.Zero(t, r.exitCode)

		resolvConf := []byte(strings.TrimSpace(r.stdout))

		if dns != nil {
			if len(dns.Servers) > 0 {
				require.ElementsMatch(t, dns.Servers, dresolvconf.GetNameservers(resolvConf, dtypes.IP))
			}
			if len(dns.Searches) > 0 {
				require.ElementsMatch(t, dns.Searches, dresolvconf.GetSearchDomains(resolvConf))
			}
			if len(dns.Options) > 0 {
				require.ElementsMatch(t, dns.Options, dresolvconf.GetOptions(resolvConf))
			}
		} else {
			system, err := dresolvconf.Get()
			require.NoError(t, err)
			require.ElementsMatch(t, dresolvconf.GetNameservers(system.Content, dtypes.IP), dresolvconf.GetNameservers(resolvConf, dtypes.IP))
			require.ElementsMatch(t, dresolvconf.GetSearchDomains(system.Content), dresolvconf.GetSearchDomains(resolvConf))
			require.ElementsMatch(t, dresolvconf.GetOptions(system.Content), dresolvconf.GetOptions(resolvConf))
		}
	})
}
