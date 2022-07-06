package acl

import (
	"fmt"
	"sort"
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix"
	glob "github.com/ryanuber/go-glob"
)

// Redefine this value from structs to avoid circular dependency.
const AllNamespacesSentinel = "*"

// ManagementACL is a singleton used for management tokens
var ManagementACL *ACL

func init() {
	var err error
	ManagementACL, err = NewACL(true, nil)
	if err != nil {
		panic(fmt.Errorf("failed to setup management ACL: %v", err))
	}
}

// capabilitySet is a type wrapper to help managing a set of capabilities
type capabilitySet map[string]struct{}

func (c capabilitySet) Check(k string) bool {
	_, ok := c[k]
	return ok
}

func (c capabilitySet) Set(k string) {
	c[k] = struct{}{}
}

func (c capabilitySet) Clear() {
	for cap := range c {
		delete(c, cap)
	}
}

// ACL object is used to convert a set of policies into a structure that
// can be efficiently evaluated to determine if an action is allowed.
type ACL struct {
	// management tokens are allowed to do anything
	management bool

	// namespaces maps a namespace to a capabilitySet
	namespaces *iradix.Tree

	// wildcardNamespaces maps a glob pattern of a namespace to a capabilitySet
	// We use an iradix for the purposes of ordered iteration.
	wildcardNamespaces *iradix.Tree

	// hostVolumes maps a named host volume to a capabilitySet
	hostVolumes *iradix.Tree

	// wildcardHostVolumes maps a glob pattern of host volume names to a capabilitySet
	// We use an iradix for the purposes of ordered iteration.
	wildcardHostVolumes *iradix.Tree

	agent    string
	node     string
	operator string
	quota    string
	plugin   string
}

// maxPrivilege returns the policy which grants the most privilege
// This handles the case of Deny always taking maximum precedence.
func maxPrivilege(a, b string) string {
	switch {
	case a == PolicyDeny || b == PolicyDeny:
		return PolicyDeny
	case a == PolicyWrite || b == PolicyWrite:
		return PolicyWrite
	case a == PolicyRead || b == PolicyRead:
		return PolicyRead
	case a == PolicyList || b == PolicyList:
		return PolicyList
	default:
		return ""
	}
}

// NewACL compiles a set of policies into an ACL object
func NewACL(management bool, policies []*Policy) (*ACL, error) {
	// Hot-path management tokens
	if management {
		return &ACL{management: true}, nil
	}

	// Create the ACL object
	acl := &ACL{}
	nsTxn := iradix.New().Txn()
	wnsTxn := iradix.New().Txn()
	hvTxn := iradix.New().Txn()
	whvTxn := iradix.New().Txn()

	for _, policy := range policies {
	NAMESPACES:
		for _, ns := range policy.Namespaces {
			// Should the namespace be matched using a glob?
			globDefinition := strings.Contains(ns.Name, "*")

			// Check for existing capabilities
			var capabilities capabilitySet

			if globDefinition {
				raw, ok := wnsTxn.Get([]byte(ns.Name))
				if ok {
					capabilities = raw.(capabilitySet)
				} else {
					capabilities = make(capabilitySet)
					wnsTxn.Insert([]byte(ns.Name), capabilities)
				}
			} else {
				raw, ok := nsTxn.Get([]byte(ns.Name))
				if ok {
					capabilities = raw.(capabilitySet)
				} else {
					capabilities = make(capabilitySet)
					nsTxn.Insert([]byte(ns.Name), capabilities)
				}
			}

			// Deny always takes precedence
			if capabilities.Check(NamespaceCapabilityDeny) {
				continue NAMESPACES
			}

			// Add in all the capabilities
			for _, cap := range ns.Capabilities {
				if cap == NamespaceCapabilityDeny {
					// Overwrite any existing capabilities
					capabilities.Clear()
					capabilities.Set(NamespaceCapabilityDeny)
					continue NAMESPACES
				}
				capabilities.Set(cap)
			}
		}

	HOSTVOLUMES:
		for _, hv := range policy.HostVolumes {
			// Should the volume be matched using a glob?
			globDefinition := strings.Contains(hv.Name, "*")

			// Check for existing capabilities
			var capabilities capabilitySet

			if globDefinition {
				raw, ok := whvTxn.Get([]byte(hv.Name))
				if ok {
					capabilities = raw.(capabilitySet)
				} else {
					capabilities = make(capabilitySet)
					whvTxn.Insert([]byte(hv.Name), capabilities)
				}
			} else {
				raw, ok := hvTxn.Get([]byte(hv.Name))
				if ok {
					capabilities = raw.(capabilitySet)
				} else {
					capabilities = make(capabilitySet)
					hvTxn.Insert([]byte(hv.Name), capabilities)
				}
			}

			// Deny always takes precedence
			if capabilities.Check(HostVolumeCapabilityDeny) {
				continue
			}

			// Add in all the capabilities
			for _, cap := range hv.Capabilities {
				if cap == HostVolumeCapabilityDeny {
					// Overwrite any existing capabilities
					capabilities.Clear()
					capabilities.Set(HostVolumeCapabilityDeny)
					continue HOSTVOLUMES
				}
				capabilities.Set(cap)
			}
		}

		// Take the maximum privilege for agent, node, and operator
		if policy.Agent != nil {
			acl.agent = maxPrivilege(acl.agent, policy.Agent.Policy)
		}
		if policy.Node != nil {
			acl.node = maxPrivilege(acl.node, policy.Node.Policy)
		}
		if policy.Operator != nil {
			acl.operator = maxPrivilege(acl.operator, policy.Operator.Policy)
		}
		if policy.Quota != nil {
			acl.quota = maxPrivilege(acl.quota, policy.Quota.Policy)
		}
		if policy.Plugin != nil {
			acl.plugin = maxPrivilege(acl.plugin, policy.Plugin.Policy)
		}
	}

	// Finalize the namespaces
	acl.namespaces = nsTxn.Commit()
	acl.wildcardNamespaces = wnsTxn.Commit()
	acl.hostVolumes = hvTxn.Commit()
	acl.wildcardHostVolumes = whvTxn.Commit()

	return acl, nil
}

// AllowNsOp is shorthand for AllowNamespaceOperation
func (a *ACL) AllowNsOp(ns string, op string) bool {
	return a.AllowNamespaceOperation(ns, op)
}

// AllowNsOpFunc is a helper that returns a function that can be used to check
// namespace permissions.
func (a *ACL) AllowNsOpFunc(ops ...string) func(string) bool {
	return func(ns string) bool {
		return NamespaceValidator(ops...)(a, ns)
	}
}

// AllowNamespaceOperation checks if a given operation is allowed for a namespace.
func (a *ACL) AllowNamespaceOperation(ns string, op string) bool {
	// Hot path if ACL is not enabled.
	if a == nil {
		return true
	}

	// Hot path management tokens
	if a.management {
		return true
	}

	// If using the all namespaces wildcard, allow if any namespace allows the
	// operation.
	if ns == AllNamespacesSentinel && a.anyNamespaceAllowsOp(op) {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingNamespaceCapabilitySet(ns)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	return capabilities.Check(op)
}

// AllowNamespace checks if any operations are allowed for a namespace
func (a *ACL) AllowNamespace(ns string) bool {
	// Hot path if ACL is not enabled.
	if a == nil {
		return true
	}

	// Hot path management tokens
	if a.management {
		return true
	}

	// If using the all namespaces wildcard, allow if any namespace allows any
	// operation.
	if ns == AllNamespacesSentinel && a.anyNamespaceAllowsAnyOp() {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingNamespaceCapabilitySet(ns)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	if len(capabilities) == 0 {
		return false
	}

	return !capabilities.Check(PolicyDeny)
}

// AllowHostVolumeOperation checks if a given operation is allowed for a host volume
func (a *ACL) AllowHostVolumeOperation(hv string, op string) bool {
	// Hot path management tokens
	if a.management {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingHostVolumeCapabilitySet(hv)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	return capabilities.Check(op)
}

// AllowHostVolume checks if any operations are allowed for a HostVolume
func (a *ACL) AllowHostVolume(ns string) bool {
	// Hot path management tokens
	if a.management {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingHostVolumeCapabilitySet(ns)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	if len(capabilities) == 0 {
		return false
	}

	return !capabilities.Check(PolicyDeny)
}

// matchingNamespaceCapabilitySet looks for a capabilitySet that matches the namespace,
// if no concrete definitions are found, then we return the closest matching
// glob.
// The closest matching glob is the one that has the smallest character
// difference between the namespace and the glob.
func (a *ACL) matchingNamespaceCapabilitySet(ns string) (capabilitySet, bool) {
	// Check for a concrete matching capability set
	raw, ok := a.namespaces.Get([]byte(ns))
	if ok {
		return raw.(capabilitySet), true
	}

	// We didn't find a concrete match, so lets try and evaluate globs.
	return a.findClosestMatchingGlob(a.wildcardNamespaces, ns)
}

// anyNamespaceAllowsOp returns true if any namespace in ACL object allows the
// given operation.
func (a *ACL) anyNamespaceAllowsOp(op string) bool {
	return a.anyNamespaceAllows(func(c capabilitySet) bool {
		return c.Check(op)
	})
}

// anyNamespaceAllowsAnyOp returns true if any namespace in ACL object allows
// at least one operation.
func (a *ACL) anyNamespaceAllowsAnyOp() bool {
	return a.anyNamespaceAllows(func(c capabilitySet) bool {
		return len(c) > 0 && !c.Check(PolicyDeny)
	})
}

// anyNamespaceAllows returns true if the callback cb returns true for any
// namespace operation of the ACL object.
func (a *ACL) anyNamespaceAllows(cb func(capabilitySet) bool) bool {
	allow := false

	checkFn := func(_ []byte, iv interface{}) bool {
		v := iv.(capabilitySet)
		allow = cb(v)
		return allow
	}

	a.namespaces.Root().Walk(checkFn)
	if allow {
		return true
	}

	a.wildcardNamespaces.Root().Walk(checkFn)
	return allow
}

// matchingHostVolumeCapabilitySet looks for a capabilitySet that matches the host volume name,
// if no concrete definitions are found, then we return the closest matching
// glob.
// The closest matching glob is the one that has the smallest character
// difference between the volume name and the glob.
func (a *ACL) matchingHostVolumeCapabilitySet(name string) (capabilitySet, bool) {
	// Check for a concrete matching capability set
	raw, ok := a.hostVolumes.Get([]byte(name))
	if ok {
		return raw.(capabilitySet), true
	}

	// We didn't find a concrete match, so lets try and evaluate globs.
	return a.findClosestMatchingGlob(a.wildcardHostVolumes, name)
}

type matchingGlob struct {
	name          string
	difference    int
	capabilitySet capabilitySet
}

func (a *ACL) findClosestMatchingGlob(radix *iradix.Tree, ns string) (capabilitySet, bool) {
	// First, find all globs that match.
	matchingGlobs := findAllMatchingWildcards(radix, ns)

	// If none match, let's return.
	if len(matchingGlobs) == 0 {
		return capabilitySet{}, false
	}

	// If a single matches, lets be efficient and return early.
	if len(matchingGlobs) == 1 {
		return matchingGlobs[0].capabilitySet, true
	}

	// Stable sort the matched globs, based on the character difference between
	// the glob definition and the requested namespace. This allows us to be
	// more consistent about results based on the policy definition.
	sort.SliceStable(matchingGlobs, func(i, j int) bool {
		return matchingGlobs[i].difference <= matchingGlobs[j].difference
	})

	return matchingGlobs[0].capabilitySet, true
}

func findAllMatchingWildcards(radix *iradix.Tree, name string) []matchingGlob {
	var matches []matchingGlob

	nsLen := len(name)

	radix.Root().Walk(func(bk []byte, iv interface{}) bool {
		k := string(bk)
		v := iv.(capabilitySet)

		isMatch := glob.Glob(k, name)
		if isMatch {
			pair := matchingGlob{
				name:          k,
				difference:    nsLen - len(k) + strings.Count(k, glob.GLOB),
				capabilitySet: v,
			}
			matches = append(matches, pair)
		}

		// We always want to walk the entire tree, never terminate early.
		return false
	})

	return matches
}

// AllowAgentRead checks if read operations are allowed for an agent
func (a *ACL) AllowAgentRead() bool {
	switch {
	case a.management:
		return true
	case a.agent == PolicyWrite:
		return true
	case a.agent == PolicyRead:
		return true
	default:
		return false
	}
}

// AllowAgentWrite checks if write operations are allowed for an agent
func (a *ACL) AllowAgentWrite() bool {
	switch {
	case a.management:
		return true
	case a.agent == PolicyWrite:
		return true
	default:
		return false
	}
}

// AllowNodeRead checks if read operations are allowed for a node
func (a *ACL) AllowNodeRead() bool {
	switch {
	case a.management:
		return true
	case a.node == PolicyWrite:
		return true
	case a.node == PolicyRead:
		return true
	default:
		return false
	}
}

// AllowNodeWrite checks if write operations are allowed for a node
func (a *ACL) AllowNodeWrite() bool {
	switch {
	case a.management:
		return true
	case a.node == PolicyWrite:
		return true
	default:
		return false
	}
}

// AllowOperatorRead checks if read operations are allowed for a operator
func (a *ACL) AllowOperatorRead() bool {
	switch {
	case a.management:
		return true
	case a.operator == PolicyWrite:
		return true
	case a.operator == PolicyRead:
		return true
	default:
		return false
	}
}

// AllowOperatorWrite checks if write operations are allowed for a operator
func (a *ACL) AllowOperatorWrite() bool {
	switch {
	case a.management:
		return true
	case a.operator == PolicyWrite:
		return true
	default:
		return false
	}
}

// AllowQuotaRead checks if read operations are allowed for all quotas
func (a *ACL) AllowQuotaRead() bool {
	switch {
	case a.management:
		return true
	case a.quota == PolicyWrite:
		return true
	case a.quota == PolicyRead:
		return true
	default:
		return false
	}
}

// AllowQuotaWrite checks if write operations are allowed for quotas
func (a *ACL) AllowQuotaWrite() bool {
	switch {
	case a.management:
		return true
	case a.quota == PolicyWrite:
		return true
	default:
		return false
	}
}

// AllowPluginRead checks if read operations are allowed for all plugins
func (a *ACL) AllowPluginRead() bool {
	switch {
	// ACL is nil only if ACLs are disabled
	case a == nil:
		return true
	case a.management:
		return true
	case a.plugin == PolicyRead:
		return true
	default:
		return false
	}
}

// AllowPluginList checks if list operations are allowed for all plugins
func (a *ACL) AllowPluginList() bool {
	switch {
	// ACL is nil only if ACLs are disabled
	case a == nil:
		return true
	case a.management:
		return true
	case a.plugin == PolicyList:
		return true
	case a.plugin == PolicyRead:
		return true
	default:
		return false
	}
}

// IsManagement checks if this represents a management token
func (a *ACL) IsManagement() bool {
	return a.management
}

// NamespaceValidator returns a func that wraps ACL.AllowNamespaceOperation in
// a list of operations. Returns true (allowed) if acls are disabled or if
// *any* capabilities match.
func NamespaceValidator(ops ...string) func(*ACL, string) bool {
	return func(acl *ACL, ns string) bool {
		// Always allow if ACLs are disabled.
		if acl == nil {
			return true
		}

		for _, op := range ops {
			if acl.AllowNamespaceOperation(ns, op) {
				// An operation is allowed, return true
				return true
			}
		}

		// No operations are allowed by this ACL, return false
		return false
	}
}
