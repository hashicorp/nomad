package loglib

import (
	"crypto/sha1"
	"encoding/base32"
	"errors"
	"hash"
	"io"
	"strconv"
	"strings"
)

type LogConfig struct {
	// The identifiers for the task can be used by the log plugin to tag logs or
	// to uniquely identify a log shipper subprocess.
	JobID     string
	AllocID   string
	GroupName string
	TaskName  string

	// LogDir is the host path where logs are to be written to
	LogDir string

	// StdoutLogFile is the path relative to LogDir for stdout logging
	StdoutLogFile string

	// StderrLogFile is the path relative to LogDir for stderr logging
	StderrLogFile string

	// StdoutFifo is the path on the host to the stdout pipe
	StdoutFifo string

	// StderrFifo is the path on the host to the stderr pipe
	StderrFifo string

	// MaxFiles is the max rotated files allowed
	MaxFiles int

	// MaxFileSizeMB is the max log file size in MB allowed before rotation occures
	MaxFileSizeMB int
}

func (cfg *LogConfig) Validate() error {
	if cfg.LogDir == "" {
		return errors.New("missing LogDir")
	}
	if cfg.StdoutLogFile == "" {
		return errors.New("missing StdoutLogFile")
	}
	if cfg.StderrLogFile == "" {
		return errors.New("missing StderrLogFile")
	}
	if cfg.StdoutFifo == "" {
		return errors.New("missing StdoutFifo")
	}
	if cfg.StderrFifo == "" {
		return errors.New("missing StderrFifo")
	}

	return nil
}

var b32 = base32.NewEncoding(strings.ToLower("abcdefghijklmnopqrstuvwxyz234567"))

// ID returns a unique ID for the config, based on all its parameters
func (cfg *LogConfig) ID() string {
	h := sha1.New()

	hashString(h, cfg.JobID)
	hashString(h, cfg.AllocID)
	hashString(h, cfg.GroupName)
	hashString(h, cfg.TaskName)
	hashString(h, cfg.LogDir)
	hashString(h, cfg.StdoutLogFile)
	hashString(h, cfg.StderrLogFile)
	hashString(h, cfg.StdoutFifo)
	hashString(h, cfg.StderrFifo)
	hashInt(h, cfg.MaxFiles)
	hashInt(h, cfg.MaxFileSizeMB)

	return b32.EncodeToString(h.Sum(nil))
}

func hashString(h hash.Hash, s string) {
	_, _ = io.WriteString(h, s)
}

func hashInt(h hash.Hash, i int) {
	_, _ = io.WriteString(h, strconv.Itoa(i))
}
