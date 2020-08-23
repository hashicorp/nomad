package client

import (
	"sync"
	"time"

	log "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/client/fingerprint"
	"github.com/hashicorp/nomad/helper/pluginutils/loader"
	"github.com/hashicorp/nomad/nomad/structs"
)

// FingerprintManager runs a client fingerprinters on a continuous basis, and
// updates the client when the node has changed
type FingerprintManager struct {
	singletonLoader loader.PluginCatalog
	getConfig       func() *config.Config
	node            *structs.Node
	nodeLock        sync.Mutex
	shutdownCh      chan struct{}

	// updateNodeAttributes is a callback to the client to update the state of its
	// associated node
	updateNodeAttributes func(*fingerprint.FingerprintResponse) *structs.Node

	reloadableFps map[string]fingerprint.ReloadableFingerprint

	logger log.Logger
}

// NewFingerprintManager is a constructor that creates and returns an instance
// of FingerprintManager
func NewFingerprintManager(
	singletonLoader loader.PluginCatalog,
	getConfig func() *config.Config,
	node *structs.Node,
	shutdownCh chan struct{},
	updateNodeAttributes func(*fingerprint.FingerprintResponse) *structs.Node,
	logger log.Logger) *FingerprintManager {

	return &FingerprintManager{
		singletonLoader:      singletonLoader,
		getConfig:            getConfig,
		updateNodeAttributes: updateNodeAttributes,
		node:                 node,
		shutdownCh:           shutdownCh,
		logger:               logger.Named("fingerprint_mgr"),
		reloadableFps:        make(map[string]fingerprint.ReloadableFingerprint),
	}
}

// setNode updates the current client node
func (fm *FingerprintManager) setNode(node *structs.Node) {
	fm.nodeLock.Lock()
	defer fm.nodeLock.Unlock()
	fm.node = node
}

// getNode returns the current client node
func (fm *FingerprintManager) getNode() *structs.Node {
	fm.nodeLock.Lock()
	defer fm.nodeLock.Unlock()
	return fm.node
}

// Run starts the process of fingerprinting the node. It does an initial pass,
// identifying whitelisted and blacklisted fingerprints/drivers. Then, for
// those which require periotic checking, it starts a periodic process for
// each.
func (fp *FingerprintManager) Run() error {
	// First, set up all fingerprints
	cfg := fp.getConfig()
	whitelistFingerprints := cfg.ReadStringListToMap("fingerprint.whitelist")
	whitelistFingerprintsEnabled := len(whitelistFingerprints) > 0
	blacklistFingerprints := cfg.ReadStringListToMap("fingerprint.blacklist")

	fp.logger.Debug("built-in fingerprints", "fingerprinters", fingerprint.BuiltinFingerprints())

	var availableFingerprints []string
	var skippedFingerprints []string
	for _, name := range fingerprint.BuiltinFingerprints() {
		// Skip modules that are not in the whitelist if it is enabled.
		if _, ok := whitelistFingerprints[name]; whitelistFingerprintsEnabled && !ok {
			skippedFingerprints = append(skippedFingerprints, name)
			continue
		}
		// Skip modules that are in the blacklist
		if _, ok := blacklistFingerprints[name]; ok {
			skippedFingerprints = append(skippedFingerprints, name)
			continue
		}

		availableFingerprints = append(availableFingerprints, name)
	}

	if err := fp.setupFingerprinters(availableFingerprints); err != nil {
		return err
	}

	if len(skippedFingerprints) != 0 {
		fp.logger.Debug("fingerprint modules skipped due to white/blacklist",
			"skipped_fingerprinters", skippedFingerprints)
	}

	return nil
}

// Reload will reload any registered ReloadableFingerprinters and immediately call Fingerprint
func (fm *FingerprintManager) Reload() {
	for name, fp := range fm.reloadableFps {
		fm.logger.Info("reloading fingerprinter", "fingerprinter", name)
		fp.Reload()
		if _, err := fm.fingerprint(name, fp); err != nil {
			fm.logger.Warn("error fingerprinting after reload", "fingerprinter", name, "error", err)
		}
	}
}

// setupFingerprints is used to fingerprint the node to see if these attributes are
// supported
func (fm *FingerprintManager) setupFingerprinters(fingerprints []string) error {
	var appliedFingerprints []string

	for _, name := range fingerprints {
		f, err := fingerprint.NewFingerprint(name, fm.logger)

		if err != nil {
			fm.logger.Error("error fingerprinting", "error", err, "fingerprinter", name)
			return err
		}

		detected, err := fm.fingerprint(name, f)
		if err != nil {
			return err
		}

		// log the fingerprinters which have been applied
		if detected {
			appliedFingerprints = append(appliedFingerprints, name)
		}

		p, period := f.Periodic()
		if p {
			go fm.runFingerprint(f, period, name)
		}

		if rfp, ok := f.(fingerprint.ReloadableFingerprint); ok {
			fm.reloadableFps[name] = rfp
		}
	}

	fm.logger.Debug("detected fingerprints", "node_attrs", appliedFingerprints)
	return nil
}

// runFingerprint runs each fingerprinter individually on an ongoing basis
func (fm *FingerprintManager) runFingerprint(f fingerprint.Fingerprint, period time.Duration, name string) {
	fm.logger.Debug("fingerprinting periodically", "fingerprinter", name, "period", period)

	timer := time.NewTimer(period)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			timer.Reset(period)

			_, err := fm.fingerprint(name, f)
			if err != nil {
				fm.logger.Debug("error periodic fingerprinting", "error", err, "fingerprinter", name)
				continue
			}

		case <-fm.shutdownCh:
			return
		}
	}
}

// fingerprint does an initial fingerprint of the client. If the fingerprinter
// is meant to be run continuously, a process is launched to perform this
// fingerprint on an ongoing basis in the background.
func (fm *FingerprintManager) fingerprint(name string, f fingerprint.Fingerprint) (bool, error) {
	var response fingerprint.FingerprintResponse

	fm.nodeLock.Lock()
	request := &fingerprint.FingerprintRequest{Config: fm.getConfig(), Node: fm.node}
	err := f.Fingerprint(request, &response)
	fm.nodeLock.Unlock()

	if err != nil {
		return false, err
	}

	if node := fm.updateNodeAttributes(&response); node != nil {
		fm.setNode(node)
	}

	return response.Detected, nil
}
