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
	// Should be replaces with c.w.RawWrite(m.Severity, m.Message)
	switch m.Severity {
	case syslog.LOG_EMERG:
		return c.w.Emerg(string(m.Message))
	case syslog.LOG_ALERT:
		return c.w.Alert(string(m.Message))
	case syslog.LOG_CRIT:
		return c.w.Crit(string(m.Message))
	case syslog.LOG_ERR:
		return c.w.Err(string(m.Message))
	case syslog.LOG_WARNING:
		return c.w.Warning(string(m.Message))
	case syslog.LOG_NOTICE:
		return c.w.Notice(string(m.Message))
	case syslog.LOG_INFO:
		return c.w.Info(string(m.Message))
	default:
		return c.w.Debug(string(m.Message))
	}
}
