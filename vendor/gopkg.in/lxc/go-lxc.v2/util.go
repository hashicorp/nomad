// Copyright © 2013, 2014, The Go-LXC Authors. All rights reserved.
// Use of this source code is governed by a LGPLv2.1
// license that can be found in the LICENSE file.

// +build linux,cgo

package lxc

/*
#include <stdlib.h>
#include <lxc/lxccontainer.h>

static char** makeCharArray(size_t size) {
    // caller checks return value
    return calloc(size, sizeof(char*));
}

static void setArrayString(char **array, char *string, size_t n) {
    array[n] = string;
}

static void freeCharArray(char **array, size_t size) {
    size_t i;
    for (i = 0; i < size; i++) {
        free(array[i]);
    }
    free(array);
}

static void freeSnapshotArray(struct lxc_snapshot *s, size_t size) {
    size_t i;
    for (i = 0; i < size; i++) {
        s[i].free(&s[i]);
    }
    free(s);
}

static size_t getArrayLength(char **array) {
    char **p;
    size_t size = 0;
    for (p = (char **)array; *p; p++) {
        size++;
    }
    return size;
}
*/
import "C"

import (
	"reflect"
	"unsafe"
)

func makeNullTerminatedArgs(args []string) **C.char {
	cparams := C.makeCharArray(C.size_t(len(args) + 1))
	if cparams == nil {
		return nil
	}

	for i := 0; i < len(args); i++ {
		C.setArrayString(cparams, C.CString(args[i]), C.size_t(i))
	}
	C.setArrayString(cparams, nil, C.size_t(len(args)))

	return cparams
}

func freeNullTerminatedArgs(cArgs **C.char, length int) {
	C.freeCharArray(cArgs, C.size_t(length+1))
}

func convertArgs(cArgs **C.char) []string {
	if cArgs == nil {
		return nil
	}

	return convertNArgs(cArgs, int(C.getArrayLength(cArgs)))
}

func convertNArgs(cArgs **C.char, size int) []string {
	if cArgs == nil || size <= 0 {
		return nil
	}

	var A []*C.char

	hdr := reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(cArgs)),
		Len:  size,
		Cap:  size,
	}
	cArgsInterface := reflect.NewAt(reflect.TypeOf(A), unsafe.Pointer(&hdr)).Elem().Interface()

	result := make([]string, size)
	for i := 0; i < size; i++ {
		result[i] = C.GoString(cArgsInterface.([]*C.char)[i])
	}
	C.freeCharArray(cArgs, C.size_t(size))

	return result
}

func freeSnapshots(snapshots *C.struct_lxc_snapshot, size int) {
	C.freeSnapshotArray(snapshots, C.size_t(size))
}
