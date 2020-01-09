package csimanager

import (
	"testing"

	"github.com/hashicorp/nomad/client/dynamicplugins"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/plugins/csi/fake"
)

func setupTestNodeInstanceManager(t *testing.T) (*fake.Client, *instanceManager) {
	tp := &fake.Client{}

	logger := testlog.HCLogger(t)
	pinfo := &dynamicplugins.PluginInfo{
		Name: "test-plugin",
	}

	return tp, &instanceManager{
		logger: logger,
		info:   pinfo,
		client: tp,
		fp: &pluginFingerprinter{
			logger:          logger.Named("fingerprinter"),
			info:            pinfo,
			client:          tp,
			fingerprintNode: true,
		},
	}
}
