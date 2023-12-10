// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestChecks_NomadCheckID(t *testing.T) {
	ci.Parallel(t)

	orig := ServiceCheck{
		Name:        "c1",
		Type:        "http",
		Path:        "/health",
		Protocol:    "https",
		PortLabel:   "web",
		AddressMode: "host",
		Interval:    1 * time.Minute,
		Timeout:     10 * time.Second,
		Method:      "GET",
		TaskName:    "t1",
		OnUpdate:    OnUpdateIgnore,
	}

	different := func(a, b ServiceCheck) bool {
		idA := NomadCheckID("id", "group", &a)
		idB := NomadCheckID("id", "group", &b)
		return idA != idB
	}

	t.Run("same", func(t *testing.T) {
		c := orig
		must.False(t, different(orig, c))
	})

	t.Run("different name", func(t *testing.T) {
		c := orig
		c.Name = "c2"
		must.True(t, different(orig, c))
	})

	t.Run("different type", func(t *testing.T) {
		c := orig
		c.Type = "tcp"
		must.True(t, different(orig, c))
	})

	t.Run("different path", func(t *testing.T) {
		c := orig
		c.Path = "/metrics"
		must.True(t, different(orig, c))
	})

	t.Run("different protocol", func(t *testing.T) {
		c := orig
		c.Protocol = "http"
		must.True(t, different(orig, c))
	})

	t.Run("different port label", func(t *testing.T) {
		c := orig
		c.PortLabel = "ingress"
		must.True(t, different(orig, c))
	})

	t.Run("different address mode", func(t *testing.T) {
		c := orig
		c.AddressMode = "bridge"
		must.True(t, different(orig, c))
	})

	t.Run("different interval", func(t *testing.T) {
		c := orig
		c.Interval = 1 * time.Second
		must.True(t, different(orig, c))
	})

	t.Run("different timeout", func(t *testing.T) {
		c := orig
		c.Timeout = 5 * time.Second
		must.True(t, different(orig, c))
	})

	t.Run("different method", func(t *testing.T) {
		c := orig
		c.Method = "POST"
		must.True(t, different(orig, c))
	})

	t.Run("different task", func(t *testing.T) {
		c := orig
		c.TaskName = "task2"
		must.True(t, different(orig, c))
	})

	t.Run("different on update", func(t *testing.T) {
		c := orig
		c.OnUpdate = "checks"
		must.True(t, different(orig, c))
	})
}
