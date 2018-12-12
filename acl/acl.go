package acl

import (
	"fmt"
	"sort"
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix"
	glob "github.com/ryanuber/go-glob"
)

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

	agent    string
	node     string
	operator string
	quota    string
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
	}

	// Finalize the namespaces
	acl.namespaces = nsTxn.Commit()
	acl.wildcardNamespaces = wnsTxn.Commit()
	return acl, nil
}

// AllowNsOp is shorthand for AllowNamespaceOperation
func (a *ACL) AllowNsOp(ns string, op string) bool {
	return a.AllowNamespaceOperation(ns, op)
}

// AllowNamespaceOperation checks if a given operation is allowed for a namespace
func (a *ACL) AllowNamespaceOperation(ns string, op string) bool {
	// Hot path management tokens
	if a.management {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingCapabilitySet(ns)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	return capabilities.Check(op)
}

// AllowNamespace checks if any operations are allowed for a namespace
func (a *ACL) AllowNamespace(ns string) bool {
	// Hot path management tokens
	if a.management {
		return true
	}

	// Check for a matching capability set
	capabilities, ok := a.matchingCapabilitySet(ns)
	if !ok {
		return false
	}

	// Check if the capability has been granted
	if len(capabilities) == 0 {
		return false
	}

	return !capabilities.Check(PolicyDeny)
}

// matchingCapabilitySet looks for a capabilitySet that matches the namespace,
// if no concrete definitions are found, then we return the closest matching
// glob.
// The closest matching glob is the one that has the smallest character
// difference between the namespace and the glob.
func (a *ACL) matchingCapabilitySet(ns string) (capabilitySet, bool) {
	// Check for a concrete matching capability set
	raw, ok := a.namespaces.Get([]byte(ns))
	if ok {
		return raw.(capabilitySet), true
	}

	// We didn't find a concrete match, so lets try and evaluate globs.
	return a.findClosestMatchingGlob(ns)
}

type matchingGlob struct {
	ns            string
	difference    int
	capabilitySet capabilitySet
}

func (a *ACL) findClosestMatchingGlob(ns string) (capabilitySet, bool) {
	// First, find all globs that match.
	matchingGlobs := a.findAllMatchingWildcards(ns)

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

func (a *ACL) findAllMatchingWildcards(ns string) []matchingGlob {
	var matches []matchingGlob

	nsLen := len(ns)

	a.wildcardNamespaces.Root().Walk(func(bk []byte, iv interface{}) bool {
		k := string(bk)
		v := iv.(capabilitySet)

		isMatch := glob.Glob(k, ns)
		if isMatch {
			pair := matchingGlob{
				ns:            k,
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

// IsManagement checks if this represents a management token
func (a *ACL) IsManagement() bool {
	return a.management
}
