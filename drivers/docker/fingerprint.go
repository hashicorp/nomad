package docker

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/plugins/drivers"
)

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *Driver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	defer close(ch)
	ticker := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			ticker.Reset(fingerprintPeriod)
			ch <- d.buildFingerprint()
		}
	}
}

func (d *Driver) buildFingerprint() *drivers.Fingerprint {
	fp := &drivers.Fingerprint{
		Attributes:        map[string]string{},
		Health:            drivers.HealthStateHealthy,
		HealthDescription: "healthy",
	}
	client, _, err := d.dockerClients()
	if err != nil {
		d.logger.Info("failed to initialize client", "error", err)
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUndetected,
			HealthDescription: "failed to initialize docker client",
		}
	}

	env, err := client.Version()
	if err != nil {
		d.logger.Debug("could not connect to docker daemon", "endpoint", client.Endpoint(), "error", err)
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUnhealthy,
			HealthDescription: "failed to connect to docker daemon",
		}
	}

	fp.Attributes["driver.docker"] = "1"
	fp.Attributes["driver.docker.version"] = env.Get("Version")
	if d.config.AllowPrivileged {
		fp.Attributes["driver.docker.privileged.enabled"] = "1"
	}

	if d.config.Volumes.Enabled {
		fp.Attributes["driver.docker.volumes.enabled"] = "1"
	}

	if nets, err := client.ListNetworks(); err != nil {
		d.logger.Warn("error discovering bridge IP", "error", err)
	} else {
		for _, n := range nets {
			if n.Name != "bridge" {
				continue
			}

			if len(n.IPAM.Config) == 0 {
				d.logger.Warn("no IPAM config for bridge network")
				break
			}

			if n.IPAM.Config[0].Gateway != "" {
				fp.Attributes["driver.docker.bridge_ip"] = n.IPAM.Config[0].Gateway
			} else {
				// Docker 17.09.0-ce dropped the Gateway IP from the bridge network
				// See https://github.com/moby/moby/issues/32648
				d.logger.Debug("bridge_ip could not be discovered")
			}
			break
		}
	}

	return fp
}
