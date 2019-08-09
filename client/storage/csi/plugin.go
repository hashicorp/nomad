package csi

import (
	hclog "github.com/hashicorp/go-hclog"
)

// PluginClient is a struct that handles communication with a given CSI Plugin
// socket. It will lazily connect and reconnect to the socket as required.
type PluginClient struct {
	logger hclog.Logger
}
