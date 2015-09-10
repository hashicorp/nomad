package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"
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

func TestConfig_LoadConfigFile(t *testing.T) {
	// Fails if the file doesn't exist
	if _, err := LoadConfigFile("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(fh.Name())

	// Invalid content returns error
	if _, err := fh.WriteString("nope"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := LoadConfigFile(fh.Name()); err == nil {
		t.Fatalf("expected load error, got nothing")
	}

	// Valid content parses successfully
	if err := fh.Truncate(0); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := fh.Seek(0, 0); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := fh.WriteString(`{"region":"west"}`); err != nil {
		t.Fatalf("err: %s", err)
	}

	config, err := LoadConfigFile(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" {
		t.Fatalf("bad region: %q", config.Region)
	}
}

func TestConfig_LoadConfigDir(t *testing.T) {
	// Fails if the dir doesn't exist.
	if _, err := LoadConfigDir("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "conf1.hcl")
	err = ioutil.WriteFile(file1, []byte(`{"region":"west"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file2 := filepath.Join(dir, "conf2.hcl")
	err = ioutil.WriteFile(file2, []byte(`{"datacenter":"sfo"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	file3 := filepath.Join(dir, "conf3.hcl")
	err = ioutil.WriteFile(file3, []byte(`nope`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Fails if we have a bad config file
	if _, err := LoadConfigDir(dir); err == nil {
		t.Fatalf("expected load error, got nothing")
	}

	if err := os.Remove(file3); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works if configs are valid
	config, err := LoadConfigDir(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" || config.Datacenter != "sfo" {
		t.Fatalf("bad: %#v", config)
	}
}

func TestConfig_LoadConfig(t *testing.T) {
	// Fails if the target doesn't exist
	if _, err := LoadConfig("/unicorns/leprechauns"); err == nil {
		t.Fatalf("expected error, got nothing")
	}

	fh, err := ioutil.TempFile("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(fh.Name())

	if _, err := fh.WriteString(`{"region":"west"}`); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works on a config file
	config, err := LoadConfig(fh.Name())
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Region != "west" {
		t.Fatalf("bad: %#v", config)
	}

	dir, err := ioutil.TempDir("", "nomad")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(dir)

	file1 := filepath.Join(dir, "config1.hcl")
	err = ioutil.WriteFile(file1, []byte(`{"datacenter":"sfo"}`), 0600)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Works on config dir
	config, err = LoadConfig(dir)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if config.Datacenter != "sfo" {
		t.Fatalf("bad: %#v", config)
	}
}
