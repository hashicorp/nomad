package shipper

import (
	"flag"
	"fmt"
	"os"

	hclog "github.com/hashicorp/go-hclog"

	loglib "github.com/hashicorp/nomad/plugins/logging/lib"
)

// This init() must be initialized last in package required by the child plugin
// process. It's recommended to avoid any other `init()` or inline any necessary calls
// here. See eeaa95d commit message for more details.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "logshipper" {

		var (
			jobID, allocID, groupName, taskName                          string
			logDir, stdoutLogFile, stderrLogFile, stdoutFifo, stderrFifo string
			maxFiles, maxFileSizeMB                                      int
		)
		flags := flag.NewFlagSet("logshipper", flag.ExitOnError)

		flags.StringVar(&jobID, "job-id", "", "")
		flags.StringVar(&allocID, "alloc-id", "", "")
		flags.StringVar(&groupName, "group-name", "", "")
		flags.StringVar(&taskName, "task-name", "", "")
		flags.StringVar(&logDir, "log-dir", "", "")
		flags.StringVar(&stdoutLogFile, "stdout-log-file", "", "")
		flags.StringVar(&stderrLogFile, "stderr-log-file", "", "")
		flags.StringVar(&stdoutFifo, "stdout-fifo", "", "")
		flags.StringVar(&stderrFifo, "stderr-fifo", "", "")
		flags.IntVar(&maxFiles, "max-files", 0, "")
		flags.IntVar(&maxFileSizeMB, "max-file-size", 0, "")

		flags.Parse(os.Args[2:])

		// note: this logger will write back to the logging plugin's task
		// runner, so we want to avoid spamming this logger
		logger := hclog.New(&hclog.LoggerOptions{
			Name:       "logshipper",
			Level:      hclog.Trace,
			Output:     os.Stdout,
			JSONFormat: false,
		})

		logger.Info(fmt.Sprintf("received os.Args: %s", os.Args))

		cfg := &loglib.LogConfig{
			JobID:         jobID,
			AllocID:       allocID,
			GroupName:     groupName,
			TaskName:      taskName,
			LogDir:        logDir,
			StdoutLogFile: stdoutLogFile,
			StderrLogFile: stderrLogFile,
			StdoutFifo:    stdoutFifo,
			StderrFifo:    stderrFifo,
			MaxFiles:      maxFiles,
			MaxFileSizeMB: maxFileSizeMB,
		}
		logger.Info(fmt.Sprintf("parsed os.Args into %#v", cfg))

		err := cfg.Validate()
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}

		taskLogger, err := loglib.NewTaskLogger(cfg, logger)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)

		}

		taskLogger.Wait()
	}
}
