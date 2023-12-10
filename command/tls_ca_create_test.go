// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCACreateCommand(t *testing.T) {
	testDir := t.TempDir()
	previousDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(testDir))
	defer os.Chdir(previousDirectory)

	type testcase struct {
		name       string
		args       []string
		caPath     string
		keyPath    string
		extraCheck func(t *testing.T, cert *x509.Certificate)
	}
	// The following subtests must run serially.
	cases := []testcase{
		{"ca defaults",
			nil,
			"nomad-agent-ca.pem",
			"nomad-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, 1825*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				require.False(t, cert.PermittedDNSDomainsCritical)
				require.Len(t, cert.PermittedDNSDomains, 0)
			},
		},
		{"ca options",
			[]string{
				"-days=365",
				"-name-constraint=true",
				"-domain=foo",
				"-additional-domain=bar",
				"-common-name=CustomCA",
				"-country=ZZ",
				"-organization=CustOrg",
				"-organizational-unit=CustOrgUnit",
			},
			"foo-agent-ca.pem",
			"foo-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, 365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				require.True(t, cert.PermittedDNSDomainsCritical)
				require.Len(t, cert.PermittedDNSDomains, 4)
				require.ElementsMatch(t, cert.PermittedDNSDomains, []string{"nomad", "foo", "localhost", "bar"})
				require.Equal(t, cert.Issuer.Organization, []string{"CustOrg"})
				require.Equal(t, cert.Issuer.OrganizationalUnit, []string{"CustOrgUnit"})
				require.Equal(t, cert.Issuer.Country, []string{"ZZ"})
				require.Contains(t, cert.Issuer.CommonName, "CustomCA")
			},
		},
		{"ca custom date",
			[]string{
				"-days=365",
			},
			"nomad-agent-ca.pem",
			"nomad-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, 365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &TLSCACreateCommand{Meta: Meta{Ui: ui}}
			require.Equal(t, 0, cmd.Run(tc.args), ui.ErrorWriter.String())
			require.Equal(t, "", ui.ErrorWriter.String())
			// is a valid key
			key := testutil.IsValidSigner(t, tc.keyPath)
			require.True(t, key)
			// is a valid ca expects the ca
			ca := testutil.IsValidCertificate(t, tc.caPath)
			require.True(t, ca.BasicConstraintsValid)
			require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, ca.KeyUsage)
			require.True(t, ca.IsCA)
			require.Equal(t, ca.AuthorityKeyId, ca.SubjectKeyId)
			tc.extraCheck(t, ca)
			require.NoError(t, os.Remove(tc.caPath))
			require.NoError(t, os.Remove(tc.keyPath))
		})
	}
}
