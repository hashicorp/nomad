package e2eutil

type LogStream int

const (
	LogsStdErr LogStream = iota
	LogsStdOut
)

func AllocLogs(allocID string, logStream LogStream) (string, error) {
	cmd := []string{"nomad", "alloc", "logs"}
	if logStream == LogsStdErr {
		cmd = append(cmd, "-stderr")
	}
	cmd = append(cmd, allocID)
	return Command(cmd[0], cmd[1:]...)
}

func AllocTaskLogs(allocID, task string, logStream LogStream) (string, error) {
	cmd := []string{"nomad", "alloc", "logs"}
	if logStream == LogsStdErr {
		cmd = append(cmd, "-stderr")
	}
	cmd = append(cmd, allocID, task)
	return Command(cmd[0], cmd[1:]...)
}
