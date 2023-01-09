package oidc

import (
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestBinder_Bind(t *testing.T) {
	ci.Parallel(t)

	testStore := state.TestStateStore(t)
	testBind := NewBinder(testStore)

	// create an authMethod method and insert into the state store
	authMethod := mock.ACLAuthMethod()
	must.NoError(t, testStore.UpsertACLAuthMethods(0, []*structs.ACLAuthMethod{authMethod}))

	// create some roles and insert into the state store
	targetRole := &structs.ACLRole{
		ID:   uuid.Generate(),
		Name: "vim-role",
	}
	otherRole := &structs.ACLRole{
		ID:   uuid.Generate(),
		Name: "frontend-engineers",
	}
	must.NoError(t, testStore.UpsertACLRoles(
		structs.MsgTypeTestSetup, 0, []*structs.ACLRole{targetRole, otherRole}, true,
	))

	// create binding rules and insert into the state store
	bindingRules := []*structs.ACLBindingRule{
		{
			ID:         uuid.Generate(),
			Selector:   "role==engineer",
			BindType:   structs.ACLBindingRuleBindTypeRole,
			BindName:   "${editor}-role",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         uuid.Generate(),
			Selector:   "role==engineer",
			BindType:   structs.ACLBindingRuleBindTypeRole,
			BindName:   "this-role-does-not-exist",
			AuthMethod: authMethod.Name,
		},
		{
			ID:         uuid.Generate(),
			Selector:   "language==js",
			BindType:   structs.ACLBindingRuleBindTypeRole,
			BindName:   otherRole.Name,
			AuthMethod: authMethod.Name,
		},
	}
	must.NoError(t, testStore.UpsertACLBindingRules(0, bindingRules, true))

	tests := []struct {
		name       string
		authMethod *structs.ACLAuthMethod
		identity   *Identity
		want       *Bindings
		wantErr    bool
	}{
		{
			"empty identity",
			authMethod,
			&Identity{},
			&Bindings{},
			false,
		},
		{
			"role",
			authMethod,
			&Identity{
				Claims: map[string]string{
					"role":     "engineer",
					"language": "go",
				},
				ClaimMappings: map[string]string{
					"editor": "vim",
				},
			},
			&Bindings{Roles: []*structs.ACLTokenRoleLink{{ID: targetRole.ID}}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testBind.Bind(tt.authMethod, tt.identity)
			if tt.wantErr {
				must.Error(t, err)
			} else {
				must.NoError(t, err)
			}
			must.Eq(t, got, tt.want)
		})
	}
}

func Test_computeBindName(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name          string
		bindType      string
		bindName      string
		claimMappings map[string]string
		wantName      string
		wantTrue      bool
		wantErr       bool
	}{
		{
			"valid bind name and type",
			structs.ACLBindingRuleBindTypeRole,
			"cluster-admin",
			map[string]string{"cluster-admin": "root"},
			"cluster-admin",
			true,
			false,
		},
		{
			"invalid type",
			"amazing",
			"cluster-admin",
			map[string]string{"cluster-admin": "root"},
			"",
			false,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := computeBindName(tt.bindType, tt.bindName, tt.claimMappings)
			if tt.wantErr {
				must.NotNil(t, err)
			}
			must.Eq(t, got, tt.wantName)
			must.Eq(t, got1, tt.wantTrue)
		})
	}
}

func Test_doesSelectorMatch(t *testing.T) {
	ci.Parallel(t)
	tests := []struct {
		name           string
		selector       string
		selectableVars interface{}
		want           bool
	}{
		{
			"catch-all",
			"",
			nil,
			true,
		},
		{
			"valid selector but no selectable vars",
			"nomad_engineering_team in Groups",
			"",
			false,
		},
		{
			"valid selector and successful evaluation",
			"nomad_engineering_team in Groups",
			map[string][]string{"Groups": {"nomad_sales_team", "nomad_engineering_team"}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			must.Eq(t, doesSelectorMatch(tt.selector, tt.selectableVars), tt.want)
		})
	}
}
