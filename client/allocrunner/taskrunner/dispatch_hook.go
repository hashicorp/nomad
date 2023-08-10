// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package taskrunner

import (
	"context"
	"os"
	"path/filepath"

	"github.com/golang/snappy"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/allocrunner/interfaces"
	"github.com/hashicorp/nomad/nomad/structs"
)

// dispatchHook writes a dispatch payload to the task dir
type dispatchHook struct {
	payload []byte

	logger hclog.Logger
}

func newDispatchHook(alloc *structs.Allocation, logger hclog.Logger) *dispatchHook {
	h := &dispatchHook{
		payload: alloc.Job.Payload,
	}
	h.logger = logger.Named(h.Name())
	return h
}

func (*dispatchHook) Name() string {
	// Copied in client/state when upgrading from <0.9 schemas, so if you
	// change it here you also must change it there.
	return "dispatch_payload"
}

func (h *dispatchHook) Prestart(ctx context.Context, req *interfaces.TaskPrestartRequest, resp *interfaces.TaskPrestartResponse) error {
	if len(h.payload) == 0 || req.Task.DispatchPayload == nil || req.Task.DispatchPayload.File == "" {
		// No dispatch payload
		resp.Done = true
		return nil
	}

	err := writeDispatchPayload(req.TaskDir.LocalDir, req.Task.DispatchPayload.File, h.payload)
	if err != nil {
		return err
	}

	h.logger.Trace("dispatch payload written",
		"path", req.TaskDir.LocalDir,
		"filename", req.Task.DispatchPayload.File,
		"bytes", len(h.payload),
	)

	// Dispatch payload written successfully; mark as done
	resp.Done = true
	return nil
}

// writeDispatchPayload writes the payload to the given file or returns an
// error.
func writeDispatchPayload(base, filename string, payload []byte) error {
	renderTo := filepath.Join(base, filename)
	decoded, err := snappy.Decode(nil, payload)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(renderTo), 0777); err != nil {
		return err
	}

	return os.WriteFile(renderTo, decoded, 0777)
}
