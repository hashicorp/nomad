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
	"github.com/shoenig/test/must"
)

func TestCACreateCommand(t *testing.T) {
	testDir := t.TempDir()
	previousDirectory, err := os.Getwd()
	must.NoError(t, err)
	must.NoError(t, os.Chdir(testDir))
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
				must.Eq(t, 1825*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				must.False(t, cert.PermittedDNSDomainsCritical)
				must.SliceEmpty(t, cert.PermittedDNSDomains)
			},
		},
		{"ca custom domain",
			[]string{
				"-name-constraint=true",
				"-domain=foo.com",
			},
			"foo.com-agent-ca.pem",
			"foo.com-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				must.SliceContainsAll(t, cert.PermittedDNSDomains, []string{"nomad", "foo.com", "localhost"})
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
				must.Eq(t, 365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				must.True(t, cert.PermittedDNSDomainsCritical)
				must.Len(t, 4, cert.PermittedDNSDomains)
				must.SliceContainsAll(t, cert.PermittedDNSDomains, []string{"nomad", "foo", "localhost", "bar"})
				must.Eq(t, cert.Issuer.Organization, []string{"CustOrg"})
				must.Eq(t, cert.Issuer.OrganizationalUnit, []string{"CustOrgUnit"})
				must.Eq(t, cert.Issuer.Country, []string{"ZZ"})
				must.StrHasPrefix(t, "CustomCA", cert.Issuer.CommonName)
			},
		},
		{"ca custom date",
			[]string{
				"-days=365",
			},
			"nomad-agent-ca.pem",
			"nomad-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				must.Eq(t, 365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := &TLSCACreateCommand{Meta: Meta{Ui: ui}}
			must.Zero(t, cmd.Run(tc.args))
			must.Eq(t, "", ui.ErrorWriter.String())
			// is a valid key
			key := testutil.IsValidSigner(t, tc.keyPath)
			must.True(t, key)
			// is a valid ca expects the ca
			ca := testutil.IsValidCertificate(t, tc.caPath)
			must.True(t, ca.BasicConstraintsValid)
			must.Eq(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, ca.KeyUsage)
			must.True(t, ca.IsCA)
			must.Eq(t, ca.AuthorityKeyId, ca.SubjectKeyId)
			tc.extraCheck(t, ca)
			must.NoError(t, os.Remove(tc.caPath))
			must.NoError(t, os.Remove(tc.keyPath))
		})
	}
}
