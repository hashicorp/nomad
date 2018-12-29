package host

import (
	"fmt"
	"os"
	"testing"
)

func TestHostInfo(t *testing.T) {
	v, err := Info()
	if err != nil {
		t.Errorf("error %v", err)
	}
	empty := &InfoStat{}
	if v == empty {
		t.Errorf("Could not get hostinfo %v", v)
	}
	if v.Procs == 0 {
		t.Errorf("Could not determine the number of host processes")
	}
}

func TestUptime(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skip("Skip CI")
	}

	v, err := Uptime()
	if err != nil {
		t.Errorf("error %v", err)
	}
	if v == 0 {
		t.Errorf("Could not get up time %v", v)
	}
}

func TestBoot_time(t *testing.T) {
	if os.Getenv("CIRCLECI") == "true" {
		t.Skip("Skip CI")
	}
	v, err := BootTime()
	if err != nil {
		t.Errorf("error %v", err)
	}
	if v == 0 {
		t.Errorf("Could not get boot time %v", v)
	}
	if v < 946652400 {
		t.Errorf("Invalid Boottime, older than 2000-01-01")
	}
	t.Logf("first boot time: %d", v)

	v2, err := BootTime()
	if v != v2 {
		t.Errorf("cached boot time is different")
	}
	t.Logf("second boot time: %d", v2)
}

func TestUsers(t *testing.T) {
	v, err := Users()
	if err != nil {
		t.Errorf("error %v", err)
	}
	empty := UserStat{}
	if len(v) == 0 {
		t.Fatal("Users is empty")
	}
	for _, u := range v {
		if u == empty {
			t.Errorf("Could not Users %v", v)
		}
	}
}

func TestHostInfoStat_String(t *testing.T) {
	v := InfoStat{
		Hostname: "test",
		Uptime:   3000,
		Procs:    100,
		OS:       "linux",
		Platform: "ubuntu",
		BootTime: 1447040000,
		HostID:   "edfd25ff-3c9c-b1a4-e660-bd826495ad35",
	}
	e := `{"hostname":"test","uptime":3000,"bootTime":1447040000,"procs":100,"os":"linux","platform":"ubuntu","platformFamily":"","platformVersion":"","kernelVersion":"","virtualizationSystem":"","virtualizationRole":"","hostid":"edfd25ff-3c9c-b1a4-e660-bd826495ad35"}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("HostInfoStat string is invalid: %v", v)
	}
}

func TestUserStat_String(t *testing.T) {
	v := UserStat{
		User:     "user",
		Terminal: "term",
		Host:     "host",
		Started:  100,
	}
	e := `{"user":"user","terminal":"term","host":"host","started":100}`
	if e != fmt.Sprintf("%v", v) {
		t.Errorf("UserStat string is invalid: %v", v)
	}
}

func TestHostGuid(t *testing.T) {
	hi, err := Info()
	if err != nil {
		t.Error(err)
	}
	if hi.HostID == "" {
		t.Error("Host id is empty")
	} else {
		t.Logf("Host id value: %v", hi.HostID)
	}
}

func TestTemperatureStat_String(t *testing.T) {
	v := TemperatureStat{
		SensorKey:   "CPU",
		Temperature: 1.1,
	}
	s := `{"sensorKey":"CPU","sensorTemperature":1.1}`
	if s != fmt.Sprintf("%v", v) {
		t.Errorf("TemperatureStat string is invalid")
	}
}

func TestVirtualization(t *testing.T) {
	system, role, err := Virtualization()
	if err != nil {
		t.Errorf("Virtualization() failed, %v", err)
	}
	if system == "" || role == "" {
		t.Errorf("Virtualization() retuns empty system or role:  %s, %s", system, role)
	}

	t.Logf("Virtualization(): %s, %s", system, role)
}

func TestKernelVersion(t *testing.T) {
	version, err := KernelVersion()
	if err != nil {
		t.Errorf("KernelVersion() failed, %v", err)
	}
	if version == "" {
		t.Errorf("KernelVersion() retuns empty: %s", version)
	}

	t.Logf("KernelVersion(): %s", version)
}
