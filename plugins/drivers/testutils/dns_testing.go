package testutils

import (
	"io/ioutil"
	"strings"
	"testing"

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

		resolvConf := strings.TrimSpace(r.stdout)

		if dns != nil {
			require.ElementsMatch(t, dns.Servers, getNameservers(resolvConf))
			require.ElementsMatch(t, dns.Searches, getSearchDomains(resolvConf))
			require.ElementsMatch(t, dns.Options, getOptions(resolvConf))
		} else {
			system, err := ioutil.ReadFile("/etc/resolv.conf")
			require.NoError(t, err)
			require.ElementsMatch(t, getNameservers(string(system)), getNameservers(resolvConf))
			require.ElementsMatch(t, getSearchDomains(string(system)), getSearchDomains(resolvConf))
			require.ElementsMatch(t, getOptions(string(system)), getOptions(resolvConf))
		}
	})
}

// getLines parses input into lines and strips away # comments.
func getLines(input string) []string {
	lines := strings.Split(input, "\n")
	var output []string
	for _, currentLine := range lines {
		var commentIndex = strings.Index(currentLine, "#")
		if commentIndex == -1 {
			output = append(output, currentLine)
		} else {
			output = append(output, currentLine[:commentIndex])
		}
	}
	return output
}
func getNameservers(resolvConf string) []string {
	nameservers := []string{}
	for _, line := range getLines(resolvConf) {
		if strings.HasPrefix(line, "nameserver ") {
			nameservers = append(nameservers, line[11:])
		}
	}
	return nameservers
}

func getSearchDomains(resolvConf string) []string {
	domains := []string{}
	for _, line := range getLines(resolvConf) {
		if strings.HasPrefix(line, "search ") {
			domains = strings.Fields(line[7:])
		}
	}
	return domains
}

func getOptions(resolvConf string) []string {
	options := []string{}
	for _, line := range getLines(resolvConf) {
		if strings.HasPrefix(line, "options ") {
			options = strings.Fields(line[8:])
		}
	}
	return options
}
