// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/testutil"
	"github.com/shoenig/test/must"
)

func Test_tlsMetrics(t *testing.T) {
	ci.Parallel(t)

	// Set up an in-memory metrics sink so we can inspect emitted gauges.
	inMemorySink := metrics.NewInmemSink(10*time.Millisecond, 5*time.Second)

	cfg := metrics.DefaultConfig("nomad_test")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false

	_, err := metrics.NewGlobal(cfg, inMemorySink)
	must.NoError(t, err)

	// Define the labels and build expected gauge key names. The InmemSink
	// stores gauges keyed by "name;label=value".
	labels := []metrics.Label{{Name: "host", Value: "my-host"}}

	certGaugeKey := "nomad_test.agent.tls.cert.expiration_seconds;host=my-host"
	caGaugeKey := "nomad_test.agent.tls.ca.expiration_seconds;host=my-host"

	// Create a tlsMetrics instance directly with known expiry times so we can
	// assert the emitted gauge values without depending on real certificate
	// files.
	certExpiry := time.Now().Add(24 * time.Hour)
	caExpiry := time.Now().Add(48 * time.Hour)

	tm := &tlsMetrics{
		certExpiry: certExpiry,
		caExpiry:   caExpiry,
		labels:     labels,
		logger:     testlog.HCLogger(t),
		stopCh:     make(chan struct{}),
	}

	// Start the background emit loop with a short interval. The loop emits
	// immediately on start, so the gauges should appear quickly.
	tm.start(10 * time.Millisecond)

	// Wait until the gauges appear in the sink and verify their values are
	// within a reasonable tolerance of the expected TTL.
	testutil.Wait(t, func() (bool, error) {

		sinkData := inMemorySink.Data()

		sinkData[0].RLock()
		defer sinkData[0].RUnlock()

		certGauge, certOK := sinkData[0].Gauges[certGaugeKey]
		caGauge, caOK := sinkData[0].Gauges[caGaugeKey]

		if !certOK || !caOK {
			return false, errors.New("expiry metrics not found in gauges")
		}

		// The cert expiry is 24h from now; the value should be
		// approximately 86400 seconds. Allow a generous tolerance.
		if certGauge.Value < 86300 || certGauge.Value > 86500 {
			return false, fmt.Errorf("certificate expiry %v out of bounds", certGauge.Value)
		}

		// The CA expiry is 48h from now; the value should be approximately
		// 172800 seconds.
		if caGauge.Value < 172700 || caGauge.Value > 172900 {
			return false, fmt.Errorf("CA expiry %v out of bounds", caGauge.Value)
		}

		return true, nil
	})

	// Stop the background goroutine.
	tm.stop()
}

func Test_certFileExpiry(t *testing.T) {
	ci.Parallel(t)

	t.Run("valid cert file", func(t *testing.T) {
		certFile := filepath.Join("..", "..", "helper", "tlsutil", "testdata", "regionFoo-client-nomad.pem")

		expiry, err := certFileExpiry(certFile)
		must.NoError(t, err)
		must.True(t, !expiry.IsZero(), must.Sprint("expected non-zero expiry time"))
	})

	t.Run("valid CA file", func(t *testing.T) {
		caFile := filepath.Join("..", "..", "helper", "tlsutil", "testdata", "nomad-agent-ca.pem")

		expiry, err := certFileExpiry(caFile)
		must.NoError(t, err)
		must.True(t, !expiry.IsZero(), must.Sprint("expected non-zero expiry time"))
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := certFileExpiry("/no/such/file.pem")
		must.ErrorContains(t, err, "failed to read file")
	})

	t.Run("non-PEM content", func(t *testing.T) {
		tmpDir := t.TempDir()
		badFile := filepath.Join(tmpDir, "not-a-cert.pem")
		must.NoError(t, os.WriteFile(badFile, []byte("this is not PEM data"), 0600))

		_, err := certFileExpiry(badFile)
		must.ErrorContains(t, err, "no PEM-encoded data found")
	})

	t.Run("invalid certificate in PEM block", func(t *testing.T) {
		tmpDir := t.TempDir()
		badCertFile := filepath.Join(tmpDir, "bad-cert.pem")

		// A valid PEM block but with garbage DER content that is not a real
		// X.509 certificate.
		badPEM := "-----BEGIN CERTIFICATE-----\nYWJjZGVm\n-----END CERTIFICATE-----\n"
		must.NoError(t, os.WriteFile(badCertFile, []byte(badPEM), 0600))

		_, err := certFileExpiry(badCertFile)
		must.ErrorContains(t, err, "failed to parse certificate")
	})
}
