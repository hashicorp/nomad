package executor

type LogConfig struct {
	// LogDir is the host path where logs should be written
	LogDir string

	// StdoutLogFile is the path relative to LogDir for stdout logging
	StdoutLogFile string

	// StderrLogFile is the path relative to LogDir for stderr logging
	StderrLogFile string

	// MaxFiles is the max rotated files allowed
	MaxFiles int

	// MaxFileSizeMB is the max log file size in MB allowed before rotation occures
	MaxFileSizeMB int
}
