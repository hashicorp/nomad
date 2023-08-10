// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
)

const (
	// namespaceEnabledCacheTTL is how long to cache the response from Consul
	// /v1/agent/self API, which is used to determine whether namespaces are
	// available.
	namespaceEnabledCacheTTL = 1 * time.Minute
)

// NamespacesClient is a wrapper for the Consul NamespacesAPI, that is used to
// deal with Consul OSS vs Consul Enterprise behavior in listing namespaces.
type NamespacesClient struct {
	namespacesAPI NamespaceAPI
	agentAPI      AgentAPI

	lock    sync.Mutex
	enabled bool      // namespaces requires Ent + Namespaces feature
	updated time.Time // memoize response for a while
}

// NewNamespacesClient returns a NamespacesClient backed by a NamespaceAPI.
func NewNamespacesClient(namespacesAPI NamespaceAPI, agentAPI AgentAPI) *NamespacesClient {
	return &NamespacesClient{
		namespacesAPI: namespacesAPI,
		agentAPI:      agentAPI,
	}
}

func stale(updated, now time.Time) bool {
	return now.After(updated.Add(namespaceEnabledCacheTTL))
}

func (ns *NamespacesClient) allowable(now time.Time) bool {
	ns.lock.Lock()
	defer ns.lock.Unlock()

	if !stale(ns.updated, now) {
		return ns.enabled
	}

	self, err := ns.agentAPI.Self()
	if err != nil {
		return ns.enabled
	}

	sku, ok := SKU(self)
	if !ok {
		return ns.enabled
	}

	if sku != "ent" {
		ns.enabled = false
		ns.updated = now
		return ns.enabled
	}

	ns.enabled = Namespaces(self)
	ns.updated = now
	return ns.enabled
}

// List returns a list of Consul Namespaces.
func (ns *NamespacesClient) List() ([]string, error) {
	if !ns.allowable(time.Now()) {
		// TODO(shoenig): lets return the empty string instead, that way we do not
		//   need to normalize at call sites later on
		return []string{"default"}, nil
	}

	qo := &api.QueryOptions{
		AllowStale: true,
	}
	namespaces, _, err := ns.namespacesAPI.List(qo)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		result = append(result, namespace.Name)
	}
	sort.Strings(result)
	return result, nil
}
