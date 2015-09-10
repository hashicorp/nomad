package agent

import (
	"reflect"
	"testing"
)

func TestConfig_Merge(t *testing.T) {
	c1 := &Config{
		Region:                    "region1",
		Datacenter:                "dc1",
		NodeName:                  "node1",
		DataDir:                   "/tmp/dir1",
		LogLevel:                  "INFO",
		HttpAddr:                  "127.0.0.1:4646",
		EnableDebug:               false,
		LeaveOnInt:                false,
		LeaveOnTerm:               false,
		EnableSyslog:              false,
		SyslogFacility:            "local0.info",
		DisableUpdateCheck:        false,
		DisableAnonymousSignature: false,
	}

	c2 := &Config{
		Region:                    "region2",
		Datacenter:                "dc2",
		NodeName:                  "node2",
		DataDir:                   "/tmp/dir2",
		LogLevel:                  "DEBUG",
		HttpAddr:                  "0.0.0.0:80",
		EnableDebug:               true,
		LeaveOnInt:                true,
		LeaveOnTerm:               true,
		EnableSyslog:              true,
		SyslogFacility:            "local0.debug",
		DisableUpdateCheck:        true,
		DisableAnonymousSignature: true,
	}

	result := c1.Merge(c2)
	if !reflect.DeepEqual(result, c2) {
		t.Fatalf("bad: %#v", result)
	}
}
