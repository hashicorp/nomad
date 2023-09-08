// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/plugins/drivers"
	"github.com/hashicorp/nomad/plugins/drivers/utils"
	pstructs "github.com/hashicorp/nomad/plugins/shared/structs"
)

func (d *Driver) Fingerprint(ctx context.Context) (<-chan *drivers.Fingerprint, error) {
	// Start docker reconcilers when we start fingerprinting, a workaround for
	// task drivers not having a kind of post-setup hook.
	d.danglingReconciler.Start()

	ch := make(chan *drivers.Fingerprint)
	go d.handleFingerprint(ctx, ch)
	return ch, nil
}

func (d *Driver) previouslyDetected() bool {
	d.detectedLock.RLock()
	defer d.detectedLock.RUnlock()

	return d.detected
}

func (d *Driver) setDetected(detected bool) {
	d.detectedLock.Lock()
	defer d.detectedLock.Unlock()

	d.detected = detected
}

// setFingerprintSuccess marks the driver as having fingerprinted successfully
func (d *Driver) setFingerprintSuccess() {
	d.fingerprintLock.Lock()
	d.fingerprintSuccess = pointer.Of(true)
	d.fingerprintLock.Unlock()
}

// setFingerprintFailure marks the driver as having failed fingerprinting
func (d *Driver) setFingerprintFailure() {
	d.fingerprintLock.Lock()
	d.fingerprintSuccess = pointer.Of(false)
	d.fingerprintLock.Unlock()
}

// fingerprintSuccessful returns true if the driver has
// never fingerprinted or has successfully fingerprinted
func (d *Driver) fingerprintSuccessful() bool {
	d.fingerprintLock.Lock()
	defer d.fingerprintLock.Unlock()
	return d.fingerprintSuccess == nil || *d.fingerprintSuccess
}

func (d *Driver) handleFingerprint(ctx context.Context, ch chan *drivers.Fingerprint) {
	defer close(ch)

	ticker := time.NewTimer(0)
	defer ticker.Stop()

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
		Attributes:        make(map[string]*pstructs.Attribute, 8),
		Health:            drivers.HealthStateHealthy,
		HealthDescription: drivers.DriverHealthy,
	}

	// disable if non-root on linux systems
	if runtime.GOOS == "linux" && !utils.IsUnixRoot() {
		fp.Health = drivers.HealthStateUndetected
		fp.HealthDescription = drivers.DriverRequiresRootMessage
		d.setFingerprintFailure()
		return fp
	}

	dockerClient, err := d.getDockerClient()
	if err != nil {
		if d.fingerprintSuccessful() {
			d.logger.Info("failed to initialize client", "error", err)
		}
		d.setFingerprintFailure()
		return &drivers.Fingerprint{
			Health:            drivers.HealthStateUndetected,
			HealthDescription: "Failed to initialize docker client",
		}
	}

	env, err := dockerClient.Version()
	if err != nil {
		if d.fingerprintSuccessful() {
			d.logger.Debug("could not connect to docker daemon", "endpoint", dockerClient.Endpoint(), "error", err)
		}
		d.setFingerprintFailure()

		result := drivers.HealthStateUndetected
		if d.previouslyDetected() {
			result = drivers.HealthStateUnhealthy
		}

		return &drivers.Fingerprint{
			Health:            result,
			HealthDescription: "Failed to connect to docker daemon",
		}
	}

	d.setDetected(true)
	fp.Attributes["driver.docker"] = pstructs.NewBoolAttribute(true)
	fp.Attributes["driver.docker.version"] = pstructs.NewStringAttribute(env.Get("Version"))
	if d.config.AllowPrivileged {
		fp.Attributes["driver.docker.privileged.enabled"] = pstructs.NewBoolAttribute(true)
	}

	if d.config.PidsLimit > 0 {
		fp.Attributes["driver.docker.pids.limit"] = pstructs.NewIntAttribute(d.config.PidsLimit, "")
	}

	if d.config.Volumes.Enabled {
		fp.Attributes["driver.docker.volumes.enabled"] = pstructs.NewBoolAttribute(true)
	}

	if nets, err := dockerClient.ListNetworks(); err != nil {
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
				fp.Attributes["driver.docker.bridge_ip"] = pstructs.NewStringAttribute(n.IPAM.Config[0].Gateway)
			} else if d.fingerprintSuccess == nil {
				// Docker 17.09.0-ce dropped the Gateway IP from the bridge network
				// See https://github.com/moby/moby/issues/32648
				d.logger.Debug("bridge_ip could not be discovered")
			}
			break
		}
	}

	if dockerInfo, err := dockerClient.Info(); err != nil {
		d.logger.Warn("failed to get Docker system info", "error", err)
	} else {
		runtimeNames := make([]string, 0, len(dockerInfo.Runtimes))
		for name := range dockerInfo.Runtimes {
			if d.config.GPURuntimeName == name {
				// Nvidia runtime is detected by Docker.
				// It makes possible to run GPU workloads using Docker driver on this host.
				d.gpuRuntime = true
			}
			runtimeNames = append(runtimeNames, name)
		}
		sort.Strings(runtimeNames)

		fp.Attributes["driver.docker.runtimes"] = pstructs.NewStringAttribute(
			strings.Join(runtimeNames, ","))
		fp.Attributes["driver.docker.os_type"] = pstructs.NewStringAttribute(dockerInfo.OSType)

		// If this situations arises, we are running in Windows 10 with Linux Containers enabled via VM
		if runtime.GOOS == "windows" && dockerInfo.OSType == "linux" {
			if d.fingerprintSuccessful() {
				d.logger.Warn("Docker is configured with Linux containers; switch to Windows Containers")
			}

			d.setFingerprintFailure()
			return &drivers.Fingerprint{
				Health:            drivers.HealthStateUnhealthy,
				HealthDescription: "Docker is configured with Linux containers; switch to Windows Containers",
			}
		}
	}

	d.setFingerprintSuccess()

	return fp
}
