// +build pro ent

package structs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamespace_Validate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		Test      string
		Namespace *Namespace
		Expected  string
	}{
		{
			Test: "empty name",
			Namespace: &Namespace{
				Name: "",
			},
			Expected: "invalid name",
		},
		{
			Test: "slashes in name",
			Namespace: &Namespace{
				Name: "foo/bar",
			},
			Expected: "invalid name",
		},
		{
			Test: "too long name",
			Namespace: &Namespace{
				Name: strings.Repeat("a", 200),
			},
			Expected: "invalid name",
		},
		{
			Test: "too long description",
			Namespace: &Namespace{
				Name:        "foo",
				Description: strings.Repeat("a", 300),
			},
			Expected: "description longer than",
		},
		{
			Test: "valid",
			Namespace: &Namespace{
				Name:        "foo",
				Description: "bar",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Test, func(t *testing.T) {
			err := c.Namespace.Validate()
			if err == nil {
				if c.Expected == "" {
					return
				}

				t.Fatalf("Expected error %q; got nil", c.Expected)
			} else if c.Expected == "" {
				t.Fatalf("Unexpected error %v", err)
			} else if !strings.Contains(err.Error(), c.Expected) {
				t.Fatalf("Expected error %q; got %v", c.Expected, err)
			}
		})
	}
}

func TestNamespace_SetHash(t *testing.T) {
	assert := assert.New(t)
	ns := &Namespace{
		Name:        "foo",
		Description: "bar",
	}
	out1 := ns.SetHash()
	assert.NotNil(out1)
	assert.NotNil(ns.Hash)
	assert.Equal(out1, ns.Hash)

	ns.Description = "bam"
	out2 := ns.SetHash()
	assert.NotNil(out2)
	assert.NotNil(ns.Hash)
	assert.Equal(out2, ns.Hash)
	assert.NotEqual(out1, out2)
}
