// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package license

import (
	"testing"

	census "github.com/hashicorp/go-census/schema"
	"github.com/shoenig/test/must"
)

func TestNewCensusSchema_Validate(t *testing.T) {

	schema := NewCensusSchema()

	result, err := census.Validate(schema)
	must.NoError(t, err)

	must.True(t, result)
}
