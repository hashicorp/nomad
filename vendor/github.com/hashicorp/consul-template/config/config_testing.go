package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestConfig(contents string, t *testing.T) *Config {
	f, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = f.Write([]byte(contents))
	if err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return config
}
