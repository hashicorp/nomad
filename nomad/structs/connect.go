// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"net/netip"
	"slices"
	"strconv"
)

// ConsulConfigEntries represents Consul ConfigEntry definitions from a job for
// a single Consul namespace.
type ConsulConfigEntries struct {
	Cluster     string
	Ingress     map[string]*ConsulIngressConfigEntry
	Terminating map[string]*ConsulTerminatingConfigEntry
}

// ConfigEntries accumulates the Consul Configuration Entries defined in task groups
// of j, organized by Consul namespace.
func (j *Job) ConfigEntries() map[string]*ConsulConfigEntries {
	collection := make(map[string]*ConsulConfigEntries)

	for _, tg := range j.TaskGroups {

		// accumulate config entries by namespace
		ns := tg.Consul.GetNamespace()
		if _, exists := collection[ns]; !exists {
			collection[ns] = &ConsulConfigEntries{
				Ingress:     make(map[string]*ConsulIngressConfigEntry),
				Terminating: make(map[string]*ConsulTerminatingConfigEntry),
			}
		}

		for _, service := range tg.Services {
			if service.Connect.IsGateway() {
				gateway := service.Connect.Gateway
				if ig := gateway.Ingress; ig != nil {
					collection[ns].Ingress[service.Name] = ig
					collection[ns].Cluster = service.Cluster
				} else if term := gateway.Terminating; term != nil {
					collection[ns].Terminating[service.Name] = term
					collection[ns].Cluster = service.Cluster
				}
			}
		}
	}

	return collection
}

// ConsulTransparentProxy is used to configure the Envoy sidecar for
// "transparent proxying", which creates IP tables rules inside the network
// namespace to ensure traffic flows thru the Envoy proxy
type ConsulTransparentProxy struct {

	// UID of the Envoy proxy. Defaults to the default Envoy proxy container
	// image user.
	UID string

	// OutboundPort is the Envoy proxy's outbound listener port. Inbound TCP
	// traffic hitting the PROXY_IN_REDIRECT chain will be redirected here.
	// Defaults to 15001.
	OutboundPort uint16

	// ExcludeInboundPorts is an additional set of ports will be excluded from
	// redirection to the Envoy proxy. Can be Port.Label or Port.Value. This set
	// will be added to the ports automatically excluded for the Expose.Port and
	// Check.Expose fields.
	ExcludeInboundPorts []string

	// ExcludeOutboundPorts is a set of outbound ports that will not be
	// redirected to the Envoy proxy, specified as port numbers.
	ExcludeOutboundPorts []uint16

	// ExcludeOutboundCIDRs is a set of outbound CIDR blocks that will not be
	// redirected to the Envoy proxy.
	ExcludeOutboundCIDRs []string

	// ExcludeUIDs is a set of user IDs whose network traffic will not be
	// redirected through the Envoy proxy.
	ExcludeUIDs []string

	// NoDNS disables redirection of DNS traffic to Consul DNS. By default NoDNS
	// is false and transparent proxy will direct DNS traffic to Consul DNS if
	// available on the client.
	NoDNS bool
}

func (tp *ConsulTransparentProxy) Copy() *ConsulTransparentProxy {
	if tp == nil {
		return nil
	}
	ntp := new(ConsulTransparentProxy)
	*ntp = *tp

	ntp.ExcludeInboundPorts = slices.Clone(tp.ExcludeInboundPorts)
	ntp.ExcludeOutboundPorts = slices.Clone(tp.ExcludeOutboundPorts)
	ntp.ExcludeOutboundCIDRs = slices.Clone(tp.ExcludeOutboundCIDRs)
	ntp.ExcludeUIDs = slices.Clone(tp.ExcludeUIDs)

	return ntp
}

func (tp *ConsulTransparentProxy) Validate() error {
	for _, rawCidr := range tp.ExcludeOutboundCIDRs {
		_, err := netip.ParsePrefix(rawCidr)
		if err != nil {
			// note: error returned always include parsed string
			return fmt.Errorf("could not parse transparent proxy excluded outbound CIDR as network prefix: %w", err)
		}
	}

	requireUIDisUint := func(uidRaw string) error {
		_, err := strconv.ParseUint(uidRaw, 10, 16)
		if err != nil {
			e, ok := err.(*strconv.NumError)
			if !ok {
				return fmt.Errorf("invalid user ID %q: %w", uidRaw, err)
			}
			return fmt.Errorf("invalid user ID %q: %w", uidRaw, e.Err)
		}
		return nil
	}

	if tp.UID != "" {
		if err := requireUIDisUint(tp.UID); err != nil {
			return fmt.Errorf("transparent proxy block has invalid UID field: %w", err)
		}
	}
	for _, uid := range tp.ExcludeUIDs {
		if err := requireUIDisUint(uid); err != nil {
			return fmt.Errorf("transparent proxy block has invalid ExcludeUIDs field: %w", err)
		}
	}

	// note: ExcludeInboundPorts are validated in connect validation hook
	// because we need information from the network block

	return nil
}

func (tp *ConsulTransparentProxy) Equal(o *ConsulTransparentProxy) bool {
	if tp == nil || o == nil {
		return tp == o
	}
	if tp.UID != o.UID {
		return false
	}
	if tp.OutboundPort != o.OutboundPort {
		return false
	}
	if !slices.Equal(tp.ExcludeInboundPorts, o.ExcludeInboundPorts) {
		return false
	}
	if !slices.Equal(tp.ExcludeOutboundPorts, o.ExcludeOutboundPorts) {
		return false
	}
	if !slices.Equal(tp.ExcludeOutboundCIDRs, o.ExcludeOutboundCIDRs) {
		return false
	}
	if !slices.Equal(tp.ExcludeUIDs, o.ExcludeUIDs) {
		return false
	}
	if tp.NoDNS != o.NoDNS {
		return false
	}

	return false
}
