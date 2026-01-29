// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux
// +build !linux

package config

// Default paths to search for CNI plugin binaries.
//
// For now CNI is supported only on Linux.
const DefaultCNIPath = ""
