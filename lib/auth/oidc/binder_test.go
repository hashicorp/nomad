package oidc

import (
	"reflect"
	"testing"

	"github.com/shoenig/test/must"

	"github.com/hashicorp/nomad/nomad/structs"
)

func TestBinder_Bind(t *testing.T) {
	type fields struct {
		store      BinderStateStore
		datacenter string
	}
	type args struct {
		authMethod *structs.ACLAuthMethod
		identity   *Identity
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Bindings
		wantErr bool
	}{
		// {
		// 	"bind to role",
		// }
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Binder{
				store:      tt.fields.store,
				datacenter: tt.fields.datacenter,
			}
			got, err := b.Bind(tt.args.authMethod, tt.args.identity)
			if (err != nil) != tt.wantErr {
				t.Errorf("Binder.Bind() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Binder.Bind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_computeBindName(t *testing.T) {
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
