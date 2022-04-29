package config

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
)

func TestConfigRead(t *testing.T) {
	ci.Parallel(t)

	config := Config{}

	actual := config.Read("cake")
	if actual != "" {
		t.Errorf("Expected empty string; found %s", actual)
	}

	expected := "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.Read("cake")
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}
}

func TestConfigReadDefault(t *testing.T) {
	ci.Parallel(t)

	config := Config{}

	expected := "vanilla"
	actual := config.ReadDefault("cake", expected)
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}

	expected = "chocolate"
	config.Options = map[string]string{"cake": expected}
	actual = config.ReadDefault("cake", "vanilla")
	if actual != expected {
		t.Errorf("Expected %s, found %s", expected, actual)
	}
}
