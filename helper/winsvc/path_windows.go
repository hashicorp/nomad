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
	return pathExpansions.Expand(path)
}

var pathExpansions = &windowsPathExpansions{}

type windowsPathExpansions struct {
	SystemRoot   string
	SystemDrive  string
	ProgramData  string
	ProgramFiles string
	loadErr      error
	o            sync.Once
}

func (w *windowsPathExpansions) Expand(path string) (string, error) {
	if err := w.load(); err != nil {
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

func (w *windowsPathExpansions) load() error {
	w.o.Do(func() {
		w.SystemDrive = os.Getenv("SystemDrive")
		if w.SystemDrive == "" {
			w.loadErr = fmt.Errorf("cannot detect Windows SystemDrive path")
			return
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
			w.loadErr = fmt.Errorf("cannot detect Windows ProgramData path")
			return
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
			w.loadErr = fmt.Errorf("cannot detect Windows ProgramFiles path")
			return
		}
		w.ProgramFiles = strings.ReplaceAll(w.ProgramFiles, "SystemDrive", w.SystemDrive)
	})

	return w.loadErr
}
