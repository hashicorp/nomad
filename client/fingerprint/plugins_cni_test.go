// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"os"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/shoenig/test/must"
)

func TestPluginsCNIFingerprint_Fingerprint_present(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "./test_fixtures/cni",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	attrVlan := f.(*PluginsCNIFingerprint).attribute("vlan")
	must.Eq(t, "v1.2.3", response.Attributes[attrCustom])
	must.Eq(t, "v1.0.2", response.Attributes[attrBridge])
	must.Eq(t, "v1.2.0", response.Attributes[attrVlan])
}

func TestPluginsCNIFingerprint_Fingerprint_multi(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "./test_fixtures/cni:./test_fixtures/cni2",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	attrVlan := f.(*PluginsCNIFingerprint).attribute("vlan")
	attrCustom2 := f.(*PluginsCNIFingerprint).attribute("custom2")
	must.Eq(t, "v1.2.3", response.Attributes[attrCustom])
	must.Eq(t, "v1.0.2", response.Attributes[attrBridge])
	must.Eq(t, "v9.9.9", response.Attributes[attrCustom2])
	must.Eq(t, "v1.2.0", response.Attributes[attrVlan])
}

func TestPluginsCNIFingerprint_Fingerprint_absent(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "/does/not/exist",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.False(t, response.Detected)
	attrCustom := f.(*PluginsCNIFingerprint).attribute("custom")
	attrBridge := f.(*PluginsCNIFingerprint).attribute("bridge")
	must.MapNotContainsKeys(t, response.Attributes, []string{attrCustom, attrBridge})
}

func TestPluginsCNIFingerprint_Fingerprint_empty(t *testing.T) {
	ci.Parallel(t)

	lister := func(string) ([]os.DirEntry, error) {
		// return an empty slice of directory entries
		// i.e. no plugins present
		return nil, nil
	}

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	f.(*PluginsCNIFingerprint).lister = lister
	request := &FingerprintRequest{
		Config: &config.Config{
			CNIPath: "./test_fixtures/cni",
		},
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.True(t, response.Detected)
}

func TestPluginsCNIFingerprint_Fingerprint_unset(t *testing.T) {
	ci.Parallel(t)

	f := NewPluginsCNIFingerprint(testlog.HCLogger(t))
	request := &FingerprintRequest{
		Config: new(config.Config),
	}
	response := new(FingerprintResponse)

	err := f.Fingerprint(request, response)
	must.NoError(t, err)
	must.False(t, response.Detected)
}
