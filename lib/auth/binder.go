// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Binder is responsible for collecting the ACL roles and policies to be
// assigned to a token generated as a result of "logging in" via an auth method.
//
// It does so by applying the auth method's configured binding rules.
type Binder struct {
	store BinderStateStore
}

// NewBinder creates a Binder with the given state store.
func NewBinder(store BinderStateStore) *Binder {
	return &Binder{store}
}

// BinderStateStore is the subset of state store methods used by the binder.
type BinderStateStore interface {
	GetACLBindingRulesByAuthMethod(ws memdb.WatchSet, authMethod string) (memdb.ResultIterator, error)
	GetACLRoleByName(ws memdb.WatchSet, roleName string) (*structs.ACLRole, error)
	ACLPolicyByName(ws memdb.WatchSet, name string) (*structs.ACLPolicy, error)
}

// Bindings contains the ACL roles and policies to be assigned to the created
// token.
type Bindings struct {
	Management bool
	Roles      []*structs.ACLTokenRoleLink
	Policies   []string
}

// None indicates that the resulting bindings would not give the created token
// access to any resources.
func (b *Bindings) None() bool {
	if b == nil {
		return true
	}

	return len(b.Policies) == 0 && len(b.Roles) == 0
}

// Bind collects the ACL roles and policies to be assigned to the created token.
func (b *Binder) Bind(authMethod *structs.ACLAuthMethod, identity *Identity) (*Bindings, error) {
	var (
		bindings Bindings
		err      error
	)

	// Load the auth method's binding rules.
	rulesIterator, err := b.store.GetACLBindingRulesByAuthMethod(nil, authMethod.Name)
	if err != nil {
		return nil, err
	}

	// Find the rules with selectors that match the identity's fields.
	matchingRules := []*structs.ACLBindingRule{}
	for {
		raw := rulesIterator.Next()
		if raw == nil {
			break
		}
		rule := raw.(*structs.ACLBindingRule)
		if doesSelectorMatch(rule.Selector, identity.Claims) {
			matchingRules = append(matchingRules, rule)
		}
	}
	if len(matchingRules) == 0 {
		return &bindings, nil
	}

	// Compute role or policy names by interpolating the identity's claim
	// mappings into the rule BindName templates.
	for _, rule := range matchingRules {
		bindName, valid, err := computeBindName(rule.BindType, rule.BindName, identity.ClaimMappings)
		switch {
		case err != nil:
			return nil, fmt.Errorf("cannot compute %q bind name for bind target: %w", rule.BindType, err)
		case !valid:
			return nil, fmt.Errorf("computed %q bind name for bind target is invalid: %q", rule.BindType, bindName)
		}

		switch rule.BindType {
		case structs.ACLBindingRuleBindTypeRole:
			role, err := b.store.GetACLRoleByName(nil, bindName)
			if err != nil {
				return nil, err
			}

			if role != nil {
				bindings.Roles = append(bindings.Roles, &structs.ACLTokenRoleLink{
					ID: role.ID,
				})
			}
		case structs.ACLBindingRuleBindTypePolicy:
			policy, err := b.store.ACLPolicyByName(nil, bindName)
			if err != nil {
				return nil, err
			}

			if policy != nil {
				bindings.Policies = append(bindings.Policies, policy.Name)
			}
		case structs.ACLBindingRuleBindTypeManagement:
			bindings.Management = true
			bindings.Policies = nil
			bindings.Roles = nil
			return &bindings, nil
		}
	}

	return &bindings, nil
}

// computeBindName processes the HIL for the provided bind type+name using the
// projected variables.
//
// - If the HIL is invalid ("", false, AN_ERROR) is returned.
// - If the computed name is not valid for the type ("INVALID_NAME", false, nil) is returned.
// - If the computed name is valid for the type ("VALID_NAME", true, nil) is returned.
func computeBindName(bindType, bindName string, claimMappings map[string]string) (string, bool, error) {
	bindName, err := interpolateHIL(bindName, claimMappings, true)
	if err != nil {
		return "", false, err
	}

	var valid bool
	switch bindType {
	case structs.ACLBindingRuleBindTypePolicy:
		valid = structs.ValidPolicyName.MatchString(bindName)
	case structs.ACLBindingRuleBindTypeRole:
		valid = structs.ValidACLRoleName.MatchString(bindName)
	case structs.ACLManagementToken:
		valid = true
	default:
		return "", false, fmt.Errorf("unknown binding rule bind type: %s", bindType)
	}

	return bindName, valid, nil
}

// doesSelectorMatch checks that a single selector matches the provided vars.
func doesSelectorMatch(selector string, selectableVars interface{}) bool {
	if selector == "" {
		return true // catch-all
	}

	eval, err := bexpr.CreateEvaluator(selector)
	if err != nil {
		return false // fails to match if selector is invalid
	}

	result, err := eval.Evaluate(selectableVars)
	if err != nil {
		return false // fails to match if evaluation fails
	}

	return result
}

// interpolateHIL processes the string as if it were HIL and interpolates only
// the provided string->string map as possible variables.
func interpolateHIL(s string, vars map[string]string, lowercase bool) (string, error) {
	if !strings.Contains(s, "${") {
		// Skip going to the trouble of parsing something that has no HIL.
		return s, nil
	}

	tree, err := hil.Parse(s)
	if err != nil {
		return "", err
	}

	vm := make(map[string]ast.Variable)
	for k, v := range vars {
		if lowercase {
			v = strings.ToLower(v)
		}
		vm[k] = ast.Variable{
			Type:  ast.TypeString,
			Value: v,
		}
	}

	config := &hil.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap: vm,
		},
	}

	result, err := hil.Eval(tree, config)
	if err != nil {
		return "", err
	}

	if result.Type != hil.TypeString {
		return "", fmt.Errorf("generated unexpected hil type: %s", result.Type)
	}

	return result.Value.(string), nil
}
