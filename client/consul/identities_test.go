// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestSI_DeriveTokens(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	dFunc := func(context.Context, *structs.Allocation, []string) (map[string]string, error) {
		return map[string]string{"a": "b"}, nil
	}
	tc := NewIdentitiesClient(logger, dFunc)
	tokens, err := tc.DeriveSITokens(context.TODO(), nil, nil)
	must.NoError(t, err)
	must.Eq(t, map[string]string{"a": "b"}, tokens)
}

func TestSI_DeriveTokens_error(t *testing.T) {
	ci.Parallel(t)

	logger := testlog.HCLogger(t)
	dFunc := func(context.Context, *structs.Allocation, []string) (map[string]string, error) {
		return nil, errors.New("some failure")
	}
	tc := NewIdentitiesClient(logger, dFunc)
	_, err := tc.DeriveSITokens(context.TODO(), &structs.Allocation{ID: "a1"}, nil)
	must.Error(t, err)
}
