// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"maps"
	"net"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// promSDMetaLabelPrefix prefixes every label emitted by the client
	// service discovery endpoint, following the convention used by
	// Prometheus' built-in service discovery mechanisms.
	promSDMetaLabelPrefix = "__meta_nomad_"
)

// promSDInvalidLabelChars matches characters that are not allowed in
// Prometheus label names ([a-zA-Z_][a-zA-Z0-9_]*).
var promSDInvalidLabelChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// PromSDTargetGroup is a single target group in the Prometheus HTTP service
// discovery format: https://prometheus.io/docs/prometheus/latest/http_sd/
type PromSDTargetGroup struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// ClientServiceDiscoveryRequest serves Prometheus HTTP SD target groups for
// the allocations running on the local client node. One target group is
// emitted per allocated port of every running allocation, so scrapers can
// select ports via the __meta_nomad_port_label label or the ?port= query
// parameter (e.g. ?port=metrics).
//
// Note that an unknown ?port= value is indistinguishable from "no matching
// allocations" and yields a successful empty response, which makes Prometheus
// drop all targets discovered from this endpoint. Scrape configurations
// should pair this endpoint with an absent-targets alert.
//
// This endpoint only serves local client state and is intended to be queried
// directly on each client agent, fanning scrape-target discovery out to the
// nodes instead of funneling it through the servers.
func (s *HTTPServer) ClientServiceDiscoveryRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	client := s.agent.Client()
	if client == nil {
		return nil, clientNotRunning
	}

	// Unlike the sibling /v1/client endpoints this handler serves local
	// state only; reject node_id rather than silently answering for the
	// wrong node.
	if req.URL.Query().Get("node_id") != "" {
		return nil, CodedError(http.StatusBadRequest, "node_id is not supported; query each client agent directly")
	}

	// Listing every allocation on the node spans namespaces, so require
	// node:read like the other node-level client endpoints.
	aclObj, err := s.ResolveToken(req)
	if err != nil {
		return nil, err
	}
	if !aclObj.AllowNodeRead() {
		return nil, structs.ErrPermissionDenied
	}

	portFilter := req.URL.Query().Get("port")

	node := client.Node()
	nodeLabels := map[string]string{
		promSDMetaLabelPrefix + "node_id":         client.NodeID(),
		promSDMetaLabelPrefix + "node_name":       node.Name,
		promSDMetaLabelPrefix + "node_class":      node.NodeClass,
		promSDMetaLabelPrefix + "node_pool":       node.NodePool,
		promSDMetaLabelPrefix + "node_datacenter": node.Datacenter,
	}

	return promSDTargetGroupsForAllocs(client.Allocations(), nodeLabels, portFilter, s.logger), nil
}

// promSDTargetGroupsForAllocs builds a deterministically ordered list of
// Prometheus SD target groups for the running allocations in allocs. The
// returned slice is never nil so an empty result encodes as the JSON list
// required by the HTTP SD contract rather than null.
func promSDTargetGroupsForAllocs(allocs []*structs.Allocation, nodeLabels map[string]string, portFilter string, logger hclog.Logger) []*PromSDTargetGroup {
	groups := make([]*PromSDTargetGroup, 0)
	for _, alloc := range allocs {
		if alloc.ClientStatus != structs.AllocClientStatusRunning {
			continue
		}
		// A running allocation always carries its job and allocated
		// resources; their absence means corrupted client state. Skip
		// the allocation but say so, because its targets silently
		// disappearing from a successful response is otherwise
		// undebuggable.
		if alloc.Job == nil || alloc.AllocatedResources == nil {
			logger.Warn("skipping running allocation with incomplete state in service discovery",
				"alloc_id", alloc.ID, "job_id", alloc.JobID,
				"has_job", alloc.Job != nil, "has_resources", alloc.AllocatedResources != nil)
			continue
		}
		groups = append(groups, allocPromSDTargetGroups(alloc, nodeLabels, portFilter, logger)...)
	}

	// Sort for a deterministic response body. The port value breaks ties
	// between duplicate port labels (possible across legacy task networks).
	sort.Slice(groups, func(i, j int) bool {
		gi, gj := groups[i], groups[j]
		if a, b := gi.Labels[promSDMetaLabelPrefix+"alloc_id"], gj.Labels[promSDMetaLabelPrefix+"alloc_id"]; a != b {
			return a < b
		}
		if a, b := gi.Labels[promSDMetaLabelPrefix+"port_label"], gj.Labels[promSDMetaLabelPrefix+"port_label"]; a != b {
			return a < b
		}
		return gi.Labels[promSDMetaLabelPrefix+"port"] < gj.Labels[promSDMetaLabelPrefix+"port"]
	})

	return groups
}

// allocPromSDTargetGroups builds one Prometheus SD target group per allocated
// port of the given allocation. When portFilter is non-empty only ports whose
// label matches are returned. The allocation must have a non-nil Job and
// AllocatedResources; the caller filters and logs violations.
func allocPromSDTargetGroups(alloc *structs.Allocation, nodeLabels map[string]string, portFilter string, logger hclog.Logger) []*PromSDTargetGroup {
	baseLabels := map[string]string{
		promSDMetaLabelPrefix + "namespace":   alloc.Namespace,
		promSDMetaLabelPrefix + "job_id":      alloc.JobID,
		promSDMetaLabelPrefix + "job_name":    alloc.Job.Name,
		promSDMetaLabelPrefix + "task_group":  alloc.TaskGroup,
		promSDMetaLabelPrefix + "alloc_id":    alloc.ID,
		promSDMetaLabelPrefix + "alloc_name":  alloc.Name,
		promSDMetaLabelPrefix + "alloc_index": strconv.FormatUint(uint64(alloc.Index()), 10),
	}
	maps.Copy(baseLabels, nodeLabels)

	// Expose job and task group meta (group overrides job) so schedulers
	// embedding tenant information in meta can relabel on it. The merged
	// keys are emitted in sorted order so that distinct keys colliding
	// after sanitization (e.g. "user-id" and "user_id") resolve to a
	// deterministic winner instead of flapping between polls.
	meta := alloc.Job.CombinedTaskMeta(alloc.TaskGroup, "")
	for _, k := range slices.Sorted(maps.Keys(meta)) {
		baseLabels[promSDMetaLabelPrefix+"meta_"+promSDSafeLabelName(k)] = meta[k]
	}

	var groups []*PromSDTargetGroup
	addPort := func(label, hostIP string, value int) {
		if portFilter != "" && label != portFilter {
			return
		}
		if hostIP == "" || value <= 0 {
			logger.Debug("skipping allocated port without scrapeable address in service discovery",
				"alloc_id", alloc.ID, "port_label", label, "host_ip", hostIP, "port", value)
			return
		}
		labels := make(map[string]string, len(baseLabels)+3)
		maps.Copy(labels, baseLabels)
		labels[promSDMetaLabelPrefix+"address"] = hostIP
		labels[promSDMetaLabelPrefix+"port_label"] = label
		labels[promSDMetaLabelPrefix+"port"] = strconv.Itoa(value)
		groups = append(groups, &PromSDTargetGroup{
			// JoinHostPort brackets IPv6 host IPs as Prometheus requires.
			Targets: []string{net.JoinHostPort(hostIP, strconv.Itoa(value))},
			Labels:  labels,
		})
	}

	if ports := alloc.AllocatedResources.Shared.Ports; len(ports) > 0 {
		// Modern group-level networking: ports live on Shared.Ports with
		// their bound host IP.
		for _, p := range ports {
			addPort(p.Label, p.HostIP, p.Value)
		}
		return groups
	}

	// Legacy task-level networking fallback.
	for _, task := range alloc.AllocatedResources.Tasks {
		for _, network := range task.Networks {
			for _, p := range network.DynamicPorts {
				addPort(p.Label, network.IP, p.Value)
			}
			for _, p := range network.ReservedPorts {
				addPort(p.Label, network.IP, p.Value)
			}
		}
	}
	return groups
}

// promSDSafeLabelName replaces every character not allowed in a Prometheus
// label name with an underscore.
func promSDSafeLabelName(s string) string {
	return promSDInvalidLabelChars.ReplaceAllString(s, "_")
}
