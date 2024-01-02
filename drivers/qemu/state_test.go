// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package qemu

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_taskStore(t *testing.T) {

	taskStoreImpl := newTaskStore()
	must.NotNil(t, taskStoreImpl)

	// Try reading something that doesn't exist.
	taskHandleResp1, okResp1 := taskStoreImpl.Get("this-doesn't-exist")
	must.Nil(t, taskHandleResp1)
	must.False(t, okResp1)

	// Set and get a generated task handle.
	testTaskHandle := taskHandle{pid: 131313}
	taskStoreImpl.Set("test-id", &testTaskHandle)

	taskHandleResp2, okResp2 := taskStoreImpl.Get("test-id")
	must.NotNil(t, taskHandleResp2)
	must.Eq(t, &testTaskHandle, taskHandleResp2)
	must.True(t, okResp2)

	// Delete the previously set handle, and try reading it again.
	taskStoreImpl.Delete("test-id")

	taskHandleResp3, okResp3 := taskStoreImpl.Get("test-id")
	must.Nil(t, taskHandleResp3)
	must.False(t, okResp3)

	// Deleting a non-existent handle shouldn't cause any problems.
	taskStoreImpl.Delete("test-id")
}
