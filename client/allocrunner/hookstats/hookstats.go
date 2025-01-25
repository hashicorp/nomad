// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hookstats

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
)

// Handler implements interfaces.HookStatsHandler and is used when the operator
// has not disabled hook metrics.
type Handler struct {
	baseLabels []metrics.Label
	runnerType string
}

// NewHandler creates a new hook stats handler to be used for emitting hook
// stats for operator alerting and performance identification. The base labels
// should be passed from the client set of labels and the runner type indicates
// if the hooks are run from the alloc or task runner.
func NewHandler(base []metrics.Label, runnerType string) interfaces.HookStatsHandler {
	return &Handler{
		baseLabels: base,
		runnerType: runnerType,
	}
}

func (h *Handler) Emit(start time.Time, hookName, hookType string, err error) {

	// Add the hook name to the base labels array, so we have a complete set to
	// add to the metrics. Operators do not want this as part of the metric
	// name due to cardinality control.
	labels := h.baseLabels
	labels = append(labels, metrics.Label{Name: "hook_name", Value: hookName})

	metrics.MeasureSinceWithLabels([]string{"client", h.runnerType, hookType, "elapsed"}, start, labels)
	if err != nil {
		metrics.IncrCounterWithLabels([]string{"client", h.runnerType, hookType, "failed"}, 1, labels)
	} else {
		metrics.IncrCounterWithLabels([]string{"client", h.runnerType, hookType, "success"}, 1, labels)
	}
}

// NoOpHandler implements interfaces.HookStatsHandler and is used when the
// operator has disabled hook metrics.
type NoOpHandler struct{}

func NewNoOpHandler() interfaces.HookStatsHandler { return &NoOpHandler{} }

func (n *NoOpHandler) Emit(_ time.Time, _, _ string, _ error) {}
