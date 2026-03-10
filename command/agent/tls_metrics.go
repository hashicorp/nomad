// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs/config"
)

// tlsMetrics emits metrics about TLS certificate expiry. It should be
// instantiated each time the Nomad agent loads TLS certificates into memory,
// allowing the expiry values to be read once from disk and the TTL to be
// emitted periodically.
type tlsMetrics struct {

	// certExpiry is the NotAfter time of the agent's TLS certificate.
	certExpiry time.Time

	// caExpiry is the NotAfter time of the agent's CA certificate.
	caExpiry time.Time

	// labels are the metric labels attached to each emitted gauge. They match
	// the default agent labels so operators can uniquely identify the agent.
	labels []metrics.Label

	logger hclog.Logger
	stopCh chan struct{}
}

// newTLSMetrics creates a new tlsMetrics instance that can be used to
// periodically emit TLS certificate expiry metrics. It is the callers
// responsibility to ensure the passed TLS configuration is not-nil and valid.
//
// Once created, the start and stop methods can be used to control the
// background emission of metrics. The caller should create a new instance and
// stop the old instance each time TLS certificates are reloaded.
func newTLSMetrics(logger hclog.Logger, tlsCfg *config.TLSConfig, labels []metrics.Label) (*tlsMetrics, error) {

	t := tlsMetrics{
		labels: labels,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	if tlsCfg.CertFile != "" {
		expiry, err := certFileExpiry(tlsCfg.CertFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TLS certificate file: %w", err)
		}
		t.certExpiry = expiry
	}

	if tlsCfg.CAFile != "" {
		expiry, err := certFileExpiry(tlsCfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA file: %w", err)
		}
		t.caExpiry = expiry
	}

	return &t, nil
}

// start launches the background goroutine that emits TLS certificate expiry
// metrics at regular intervals. The interval is defined by the caller and
// should be based on the agents telemetry configuration.
func (t *tlsMetrics) start(interval time.Duration) {
	t.logger.Info("starting TLS expiration metric process")
	go t.emitLoop(interval)
}

// stop signals the background goroutine to stop emitting metrics.
func (t *tlsMetrics) stop() { close(t.stopCh) }

// emitLoop periodically emits TLS certificate expiry metrics until stopped. It
// should not be called directly, but rather started and stopped using the start
// and stop methods.
func (t *tlsMetrics) emitLoop(interval time.Duration) {

	// Emit immediately on start so metrics are available without waiting for
	// the first tick interval.
	t.emit()

	ticker, tickerStop := helper.NewSafeTicker(interval)
	defer tickerStop()

	for {
		select {
		case <-ticker.C:
			t.emit()
		case <-t.stopCh:
			t.logger.Info("stopping TLS expiration metric process")
			return
		}
	}
}

// emit writes the current TLS certificate TTL metrics as gauges. The TTL is
// the number of seconds remaining until the certificate expires and will
// become negative once a certificate has expired.
func (t *tlsMetrics) emit() {
	if !t.certExpiry.IsZero() {
		metrics.SetGaugeWithLabels(
			[]string{"agent", "tls", "cert", "expiration_seconds"},
			float32(time.Until(t.certExpiry).Seconds()),
			t.labels,
		)
	}

	if !t.caExpiry.IsZero() {
		metrics.SetGaugeWithLabels(
			[]string{"agent", "tls", "ca", "expiration_seconds"},
			float32(time.Until(t.caExpiry).Seconds()),
			t.labels,
		)
	}
}

// certFileExpiry reads a PEM-encoded certificate file and returns the NotAfter
// time of the certificate.
func certFileExpiry(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return time.Time{}, errors.New("no PEM-encoded data found")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.NotAfter, nil
}
