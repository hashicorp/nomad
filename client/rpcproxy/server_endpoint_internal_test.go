package rpcproxy

import (
	"testing"
)

// func (k *EndpointKey) Equal(x *EndpointKey) {
func TestServerEndpointKey_Equal(t *testing.T) {
	tests := []struct {
		name  string
		k1    *EndpointKey
		k2    *EndpointKey
		equal bool
	}{
		{
			name:  "equal",
			k1:    &EndpointKey{name: "k1"},
			k2:    &EndpointKey{name: "k1"},
			equal: true,
		},
		{
			name:  "not equal",
			k1:    &EndpointKey{name: "k1"},
			k2:    &EndpointKey{name: "k2"},
			equal: false,
		},
	}

	for _, test := range tests {
		if test.k1.Equal(test.k2) != test.equal {
			t.Errorf("fixture %s failed forward comparison", test.name)
		}

		if test.k2.Equal(test.k1) != test.equal {
			t.Errorf("fixture %s failed reverse comparison", test.name)
		}
	}
}
