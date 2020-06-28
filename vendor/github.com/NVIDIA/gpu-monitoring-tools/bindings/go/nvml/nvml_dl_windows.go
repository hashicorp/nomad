// Copyright (c) 2015-2018, NVIDIA CORPORATION. All rights reserved.

// +build windows

package nvml

import (
	"syscall"
)

/*
#include "nvml.h"

// We wrap the call to nvmlInit() here to ensure that we pick up the correct
// version of this call. The macro magic in nvml.h that #defines the symbol
// 'nvmlInit' to 'nvmlInit_v2' is unfortunately lost on cgo.
static nvmlReturn_t nvmlInit_dl(void) {
	return nvmlInit();
}
*/
import "C"

type dlhandles struct{ handles []*syscall.LazyDLL }

var dl dlhandles

// Initialize NVML, opening a dynamic reference to the NVML library in the process.
func (dl *dlhandles) nvmlInit() C.nvmlReturn_t {
	handle := syscall.NewLazyDLL("nvml.dll")
	if handle == nil {
		return C.NVML_ERROR_LIBRARY_NOT_FOUND
	}
	dl.handles = append(dl.handles, handle)
	return C.nvmlInit_dl()
}

// Shutdown NVML, closing our dynamic reference to the NVML library in the process.
func (dl *dlhandles) nvmlShutdown() C.nvmlReturn_t {
	ret := C.nvmlShutdown()
	if ret != C.NVML_SUCCESS {
		return ret
	}

	dl.handles = dl.handles[:0]

	return C.NVML_SUCCESS
}

// Check to see if a specific symbol is present in the NVML library.
func (dl *dlhandles) lookupSymbol(symbol string) C.nvmlReturn_t {
	for _, handle := range dl.handles {
		if proc := handle.NewProc(symbol); proc != nil {
			return C.NVML_SUCCESS
		}
	}
	return C.NVML_ERROR_FUNCTION_NOT_FOUND
}
