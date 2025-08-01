// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"

	"golang.org/x/sys/windows/registry"
)

func ExpandPath(path string) (string, error) {
	if err := pathExpansions.load(); err != nil {
		return "", err
	}

	tmpl := template.New("expansion").Option("missingkey=error")
	tmpl, err := tmpl.Parse(path)
	if err != nil {
		return "", err
	}
	result := new(bytes.Buffer)
	if err := tmpl.Execute(result, pathExpansions); err != nil {
		return "", err
	}

	return result.String(), nil
}

var pathExpansions = &windowsPathExpansions{}

type windowsPathExpansions struct {
	SystemRoot   string
	SystemDrive  string
	ProgramData  string
	ProgramFiles string
	loaded       bool
	l            sync.Mutex
}

func (w *windowsPathExpansions) load() error {
	w.l.Lock()
	defer w.l.Unlock()

	if w.loaded {
		return nil
	}

	w.SystemDrive = os.Getenv("SystemDrive")
	if w.SystemDrive == "" {
		return fmt.Errorf("cannot detect Windows SystemDrive path")
	}
	w.SystemRoot = strings.ReplaceAll(os.Getenv("SystemDrive"), "SystemDrive", w.SystemDrive)

	w.ProgramData = os.Getenv("ProgramData")
	if w.ProgramData == "" {
		pdKey, err := registry.OpenKey(registry.LOCAL_MACHINE,
			`SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`, registry.QUERY_VALUE)
		if err == nil {
			if pdVal, _, err := pdKey.GetStringValue("ProgramData"); err == nil {
				w.ProgramData = pdVal
			}
		}
	}
	if w.ProgramData == "" {
		return fmt.Errorf("cannot detect Windows ProgramData path")
	}
	w.ProgramData = strings.ReplaceAll(w.ProgramData, "SystemDrive", w.SystemDrive)

	w.ProgramFiles = os.Getenv("ProgramFiles")
	if w.ProgramFiles == "" {
		pdKey, err := registry.OpenKey(registry.LOCAL_MACHINE,
			`SOFTWARE\Microsoft\Windows\CurrentVersion`, registry.QUERY_VALUE)
		if err == nil {
			if pdVal, _, err := pdKey.GetStringValue("ProgramFilesDir"); err == nil {
				w.ProgramFiles = pdVal
			}
		}
	}
	if w.ProgramFiles == "" {
		return fmt.Errorf("cannot detect Windows ProgramFiles path")
	}
	w.ProgramFiles = strings.ReplaceAll(w.ProgramFiles, "SystemDrive", w.SystemDrive)

	w.loaded = true
	return nil
}
