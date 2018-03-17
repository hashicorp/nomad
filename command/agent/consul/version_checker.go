package consul

import (
	"context"
	"log"
	"strings"
	"time"

	version "github.com/hashicorp/go-version"
)

// checkConsulTLSSkipVerify logs if Consul does not support TLSSkipVerify on
// checks and is intended to be run in a goroutine.
func checkConsulTLSSkipVerify(ctx context.Context, logger *log.Logger, client AgentAPI, done chan struct{}) {
	const (
		baseline = time.Second
		limit    = 20 * time.Second
	)

	defer close(done)

	i := uint64(0)
	for {
		self, err := client.Self()
		if err == nil {
			if supportsTLSSkipVerify(self) {
				logger.Printf("[TRACE] consul.sync: supports TLSSkipVerify")
			} else {
				logger.Printf("[WARN] consul.sync: Consul does NOT support TLSSkipVerify; please upgrade to Consul %s or newer",
					consulTLSSkipVerifyMinVersion)
			}
			return
		}

		backoff := (1 << (2 * i)) * baseline
		if backoff > limit {
			backoff = limit
		} else {
			i++
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
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
