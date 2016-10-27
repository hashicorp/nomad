package logging

import (
	"log"

	syslog "github.com/RackSec/srslog"
)

// SyslogClient is dedicated forwarder for syslog messages
type SyslogClient struct {
	w *syslog.Writer

	logger *log.Logger
}

// NewSyslogClient creates a new syslog client
func NewSyslogClient(address string, tag string, logger *log.Logger) *SyslogClient {
	if address == "" {
		logger.Printf("[INFO] external syslog receiver not defined, log forwarding dissabled")
		return nil
	}

	w, err := syslog.Dial("tcp", address, syslog.LOG_DEBUG, tag)
	if err != nil {
		logger.Printf("[ERR] syslog-client: %v", err)
		return nil
	}

	return &SyslogClient{
		w:      w,
		logger: logger,
	}
}

// Writes message to remote server
func (c *SyslogClient) Write(m *SyslogMessage) error {
	_, err := c.w.WriteWithPriority(m.Severity, m.Message)
	return err
}
