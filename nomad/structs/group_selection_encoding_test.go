// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func TestGroupSelection_HTTPJSONEncoding(t *testing.T) {
	ci.Parallel(t)
	selection := &DeploymentGroupSelection{
		Count: 3,
		Slots: map[int]*DeploymentGroupSelectionSlot{
			0: {TaskGroup: "thor", Cohort: "new", PreviousTaskGroup: "orin", PreviousCohort: "old"},
			2: nil,
		},
		RequireProgressBy: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	for _, handle := range []*codec.JsonHandle{JsonHandleWithExtensions, JsonHandlePretty} {
		for _, value := range []any{selection, *selection} {
			var buf bytes.Buffer
			must.NoError(t, codec.NewEncoder(&buf, handle).Encode(value))
			must.True(t, json.Valid(buf.Bytes()))
			var restored DeploymentGroupSelection
			must.NoError(t, json.Unmarshal(buf.Bytes(), &restored))
			must.True(t, selection.Equal(&restored))
		}
		for _, slots := range []map[int]*DeploymentGroupSelectionSlot{nil, {}, selection.Slots} {
			source := &Deployment{GroupSelections: map[string]*DeploymentGroupSelection{"runtime": {Count: selection.Count, Slots: slots, RequireProgressBy: selection.RequireProgressBy}}}
			var buf bytes.Buffer
			must.NoError(t, codec.NewEncoder(&buf, handle).Encode([]*Deployment{source}))
			must.True(t, json.Valid(buf.Bytes()))
			var restored []*Deployment
			must.NoError(t, json.Unmarshal(buf.Bytes(), &restored))
			must.Len(t, 1, restored)
			must.True(t, source.GroupSelections["runtime"].Equal(restored[0].GroupSelections["runtime"]))
			if slots == nil {
				must.Nil(t, restored[0].GroupSelections["runtime"].Slots)
			} else {
				must.NotNil(t, restored[0].GroupSelections["runtime"].Slots)
			}
		}
	}
	// Encoding must not mutate the integer-keyed persistent representation.
	must.MapLen(t, 2, selection.Slots)
	must.NotNil(t, selection.Slots[0])
}
