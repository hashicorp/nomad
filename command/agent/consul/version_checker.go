// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"strings"
	"time"

	log "github.com/hashicorp/go-hclog"
	version "github.com/hashicorp/go-version"
	"github.com/hashicorp/nomad/helper"
)

// checkConsulTLSSkipVerify logs if Consul does not support TLSSkipVerify on
// checks and is intended to be run in a goroutine.
func checkConsulTLSSkipVerify(ctx context.Context, logger log.Logger, client AgentAPI, done chan struct{}) {
	const (
		baseline = time.Second
		limit    = 20 * time.Second
	)

	defer close(done)

	timer, stop := helper.NewSafeTimer(limit)
	defer stop()

	var attempts uint64
	var backoff time.Duration

	for {
		self, err := client.Self()
		if err == nil {
			if supportsTLSSkipVerify(self) {
				logger.Trace("Consul supports TLSSkipVerify")
			} else {
				logger.Warn("Consul does NOT support TLSSkipVerify; please upgrade Consul",
					"min_version", consulTLSSkipVerifyMinVersion)
			}
			return
		}

		backoff = helper.Backoff(baseline, limit, attempts)
		attempts++
		timer.Reset(backoff)

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

var consulTLSSkipVerifyMinVersion = version.Must(version.NewVersion("0.7.2"))

// supportsTLSSkipVerify returns true if Consul supports TLSSkipVerify.
func supportsTLSSkipVerify(self map[string]map[string]interface{}) bool {
	member, ok := self["Member"]
	if !ok {
		return false
	}
	tagsI, ok := member["Tags"]
	if !ok {
		return false
	}
	tags, ok := tagsI.(map[string]interface{})
	if !ok {
		return false
	}
	buildI, ok := tags["build"]
	if !ok {
		return false
	}
	build, ok := buildI.(string)
	if !ok {
		return false
	}
	parts := strings.SplitN(build, ":", 2)
	if len(parts) != 2 {
		return false
	}
	v, err := version.NewVersion(parts[0])
	if err != nil {
		return false
	}
	if v.LessThan(consulTLSSkipVerifyMinVersion) {
		return false
	}
	return true
}
