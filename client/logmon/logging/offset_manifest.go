// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const offsetManifestVersion = 1

// OffsetManifestFile returns the log-offset manifest filename for a task log
// stream base name, for example ".web.stdout.offsets.json".
func OffsetManifestFile(baseFileName string) string {
	return fmt.Sprintf(".%s.offsets.json", baseFileName)
}

// OffsetManifestPath returns the log-offset manifest path for a task log stream.
func OffsetManifestPath(logDir, baseFileName string) string {
	return filepath.Join(filepath.Dir(logDir), OffsetManifestFile(baseFileName))
}

// OffsetManifest records the stable allocation-lifetime base offset for each
// rotated log file. File sizes may be refreshed from the filesystem when the
// manifest is read by the log streaming endpoint.
type OffsetManifest struct {
	Version      int                          `json:"version"`
	BaseFileName string                       `json:"base_file_name"`
	Files        map[string]OffsetManifestLog `json:"files"`
}

// OffsetManifestLog describes one rotated log file.
type OffsetManifestLog struct {
	Index int64 `json:"index"`
	Base  int64 `json:"base"`
	Size  int64 `json:"size"`
}

func newOffsetManifest(baseFileName string) *OffsetManifest {
	return &OffsetManifest{
		Version:      offsetManifestVersion,
		BaseFileName: baseFileName,
		Files:        make(map[string]OffsetManifestLog),
	}
}

func loadOffsetManifest(logDir, baseFileName string) (*OffsetManifest, error) {
	path := OffsetManifestPath(logDir, baseFileName)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return reconstructOffsetManifest(logDir, baseFileName)
	}
	if err != nil {
		return nil, err
	}

	var manifest OffsetManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return nil, err
	}
	if manifest.Files == nil {
		manifest.Files = make(map[string]OffsetManifestLog)
	}
	if manifest.BaseFileName == "" {
		manifest.BaseFileName = baseFileName
	}
	return &manifest, nil
}

func saveOffsetManifest(logDir, baseFileName string, manifest *OffsetManifest) error {
	path := OffsetManifestPath(logDir, baseFileName)
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func reconstructOffsetManifest(logDir, baseFileName string) (*OffsetManifest, error) {
	manifest := newOffsetManifest(baseFileName)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	type fileInfo struct {
		idx  int64
		size int64
	}
	var files []fileInfo
	prefix := baseFileName + "."
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		idx, err := strconv.ParseInt(strings.TrimPrefix(entry.Name(), prefix), 10, 64)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, fileInfo{idx: idx, size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].idx < files[j].idx })

	var base int64
	for _, file := range files {
		manifest.Files[strconv.FormatInt(file.idx, 10)] = OffsetManifestLog{
			Index: file.idx,
			Base:  base,
			Size:  file.size,
		}
		base += file.size
	}
	return manifest, nil
}
