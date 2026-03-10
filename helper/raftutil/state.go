// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	raftwal "github.com/hashicorp/raft-wal"
	"go.etcd.io/bbolt"
)

var (
	errAlreadyOpen = errors.New("unable to open raft logs that are in use")
)

// RaftStore is the interface returned by RaftStateInfo, satisfied by both
// *raftboltdb.BoltStore and *raftwal.WAL.
type RaftStore interface {
	raft.LogStore
	raft.StableStore
	Close() error
}

// RaftStateInfo returns info about the raft state found at path p. The path
// may point to a BoltDB file (raft.db) or a WAL directory. The returned
// RaftStore must be closed by the caller.
func RaftStateInfo(p string) (store RaftStore, firstIdx uint64, lastIdx uint64, err error) {
	info, statErr := os.Stat(p)
	if statErr != nil {
		return nil, 0, 0, fmt.Errorf("failed to stat %s: %v", p, statErr)
	}

	if info.IsDir() {
		return raftStateInfoWAL(p)
	}
	return raftStateInfoBoltDB(p)
}

func raftStateInfoBoltDB(p string) (store RaftStore, firstIdx uint64, lastIdx uint64, err error) {
	opts := raftboltdb.Options{
		Path: p,
		BoltOptions: &bbolt.Options{
			ReadOnly: true,
			Timeout:  1 * time.Second,
		},
		MsgpackUseNewTimeFormat: true,
	}
	s, err := raftboltdb.New(opts)
	if err != nil {
		if strings.HasSuffix(err.Error(), "timeout") {
			return nil, 0, 0, errAlreadyOpen
		}
		return nil, 0, 0, fmt.Errorf("failed to open raft logs: %v", err)
	}

	firstIdx, err = s.FirstIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch first index: %v", err)
	}

	lastIdx, err = s.LastIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch last index: %v", err)
	}

	return s, firstIdx, lastIdx, nil
}

func raftStateInfoWAL(p string) (store RaftStore, firstIdx uint64, lastIdx uint64, err error) {
	s, err := raftwal.Open(p)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to open WAL logs: %v", err)
	}

	firstIdx, err = s.FirstIndex()
	if err != nil {
		s.Close()
		return nil, 0, 0, fmt.Errorf("failed to fetch first index: %v", err)
	}

	lastIdx, err = s.LastIndex()
	if err != nil {
		s.Close()
		return nil, 0, 0, fmt.Errorf("failed to fetch last index: %v", err)
	}

	return s, firstIdx, lastIdx, nil
}

// LogEntries reads the raft logs found in the data directory found at
// the path `p`, and returns a channel of logs, and a channel of
// warnings. If opening the raft state returns an error, both channels
// will be nil.
func LogEntries(p string) (<-chan interface{}, <-chan error, error) {
	store, firstIdx, lastIdx, err := RaftStateInfo(p)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open raft logs: %v", err)
	}

	entries := make(chan interface{})
	warnings := make(chan error)

	go func() {
		defer store.Close()
		defer close(entries)
		for i := firstIdx; i <= lastIdx; i++ {
			var e raft.Log
			err := store.GetLog(i, &e)
			if err != nil {
				warnings <- fmt.Errorf(
					"failed to read log entry at index %d (firstIdx: %d, lastIdx: %d): %v",
					i, firstIdx, lastIdx, err)
				continue
			}

			entry, err := decode(&e)
			if err != nil {
				warnings <- fmt.Errorf(
					"failed to decode log entry at index %d: %v", i, err)
				continue
			}

			entries <- entry
		}
	}()

	return entries, warnings, nil
}

type logMessage struct {
	LogType string
	Term    uint64
	Index   uint64

	CommandType           string      `json:",omitempty"`
	IgnoreUnknownTypeFlag bool        `json:",omitempty"`
	Body                  interface{} `json:",omitempty"`
}

func decode(e *raft.Log) (*logMessage, error) {
	m := &logMessage{
		LogType: logTypes[e.Type],
		Term:    e.Term,
		Index:   e.Index,
	}

	if m.LogType == "" {
		m.LogType = fmt.Sprintf("%d", e.Type)
	}

	var data []byte
	if e.Type == raft.LogCommand {
		if len(e.Data) == 0 {
			return nil, fmt.Errorf("command did not include data")
		}

		msgType := structs.MessageType(e.Data[0])

		m.CommandType = commandName(msgType & ^structs.IgnoreUnknownTypeFlag)
		m.IgnoreUnknownTypeFlag = (msgType & structs.IgnoreUnknownTypeFlag) != 0

		data = e.Data[1:]
	} else {
		data = e.Data
	}

	if len(data) != 0 {
		decoder := codec.NewDecoder(bytes.NewReader(data), structs.MsgpackHandle)

		var v interface{}
		var err error
		if m.CommandType == commandName(structs.JobBatchDeregisterRequestType) {
			var vr structs.JobBatchDeregisterRequest
			err = decoder.Decode(&vr)
			v = jsonifyJobBatchDeregisterRequest(&vr)
		} else {
			var vr interface{}
			err = decoder.Decode(&vr)
			v = vr
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to decode log entry at index %d: failed to decode body of %v.%v %v\n", e.Index, e.Type, m.CommandType, err)
			v = "FAILED TO DECODE DATA"
		}
		fixTime(v)
		m.Body = v
	}

	return m, nil
}

// jsonifyJobBatchDeregisterRequest special case JsonBatchDeregisterRequest object
// as the actual type is not json friendly.
func jsonifyJobBatchDeregisterRequest(v *structs.JobBatchDeregisterRequest) interface{} {
	var data struct {
		Jobs  map[string]*structs.JobDeregisterOptions
		Evals []*structs.Evaluation
		structs.WriteRequest
	}
	data.WriteRequest = v.WriteRequest

	data.Jobs = make(map[string]*structs.JobDeregisterOptions, len(v.Jobs))
	if len(v.Jobs) != 0 {
		for k, v := range v.Jobs {
			data.Jobs[k.Namespace+"."+k.ID] = v
		}
	}
	return data
}

var logTypes = map[raft.LogType]string{
	raft.LogCommand:              "LogCommand",
	raft.LogNoop:                 "LogNoop",
	raft.LogAddPeerDeprecated:    "LogAddPeerDeprecated",
	raft.LogRemovePeerDeprecated: "LogRemovePeerDeprecated",
	raft.LogBarrier:              "LogBarrier",
	raft.LogConfiguration:        "LogConfiguration",
}

func commandName(mt structs.MessageType) string {
	n := msgTypeNames[mt]
	if n != "" {
		return n
	}

	return fmt.Sprintf("%v", mt)
}

// FindRaftStore finds a raft log store (either raft.db or wal/ directory)
// and returns the path to pass to RaftStateInfo. For BoltDB this is the
// raft.db file path; for WAL this is the wal/ directory path.
func FindRaftStore(p string) (storePath string, err error) {
	// Try WAL directories first (preferred backend), then BoltDB files,
	// at well-known locations before falling back to a filesystem walk.
	candidates := []struct {
		path  string
		isDir bool
	}{
		{filepath.Join(p, "server", "raft", "wal"), true},
		{filepath.Join(p, "raft", "wal"), true},
		{filepath.Join(p, "wal"), true},
		{filepath.Join(p, "server", "raft", "raft.db"), false},
		{filepath.Join(p, "raft", "raft.db"), false},
		{filepath.Join(p, "raft.db"), false},
	}

	for _, c := range candidates {
		info, statErr := os.Stat(c.path)
		if statErr != nil {
			continue
		}
		if c.isDir && info.IsDir() {
			return c.path, nil
		}
		if !c.isDir && !info.IsDir() {
			return c.path, nil
		}
	}

	// Accept a direct path to a .db file.
	if info, statErr := os.Stat(p); statErr == nil && !info.IsDir() && filepath.Ext(p) == ".db" {
		return p, nil
	}

	// Fall back to filesystem walk for raft.db.
	storePath, err = FindFileInPath("raft.db", p)
	if err != nil {
		return "", fmt.Errorf("no raft store (raft.db or wal/) found in %s", p)
	}
	return storePath, nil
}

// FindRaftFile finds raft.db and returns its path. This is a compatibility
// wrapper; prefer FindRaftStore for code that supports both backends.
func FindRaftFile(p string) (raftpath string, err error) {
	// Try known locations before traversal to avoid walking deep structure
	if _, err = os.Stat(filepath.Join(p, "server", "raft", "raft.db")); err == nil {
		raftpath = filepath.Join(p, "server", "raft", "raft.db")
	} else if _, err = os.Stat(filepath.Join(p, "raft", "raft.db")); err == nil {
		raftpath = filepath.Join(p, "raft", "raft.db")
	} else if _, err = os.Stat(filepath.Join(p, "raft.db")); err == nil {
		raftpath = filepath.Join(p, "raft.db")
	} else if _, err = os.Stat(p); err == nil && filepath.Ext(p) == ".db" {
		// Also accept path to .db file
		raftpath = p
	} else {
		raftpath, err = FindFileInPath("raft.db", p)
	}

	if err != nil {
		return "", err
	}

	return raftpath, nil
}

// FindRaftDir locates the raft data directory (the parent directory containing
// either raft.db or wal/). Returns the directory path regardless of backend.
func FindRaftDir(p string) (string, error) {
	storePath, err := FindRaftStore(p)
	if err != nil {
		return "", err
	}

	info, statErr := os.Stat(storePath)
	if statErr != nil {
		return "", fmt.Errorf("failed to stat raft store %s: %v", storePath, statErr)
	}

	// For WAL the store path IS a directory (wal/); return its parent.
	// For BoltDB the store path is a file (raft.db); return its parent.
	if info.IsDir() {
		return filepath.Dir(storePath), nil
	}
	return filepath.Dir(storePath), nil
}

// FindFileInPath searches for file in path p
func FindFileInPath(file string, p string) (path string, err error) {
	// Define walk function to find file
	walkFn := func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Base(walkPath) == file {
			path = walkPath
		}
		return nil
	}

	// Walk path p looking for file
	walkErr := filepath.Walk(p, walkFn)
	if walkErr != nil {
		return "", walkErr
	}

	if path == "" {
		return "", fmt.Errorf("File %s not found in path %s", file, p)
	}

	return path, nil
}
