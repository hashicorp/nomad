package command

import (
	"flag"
	"os"
	"testing"
)

func TestHelpers_HttpAddrFlag(t *testing.T) {
	var addr *string

	// Returns the default
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	addr = httpAddrFlag(flags)
	if err := flags.Parse([]string{}); err != nil {
		t.Fatalf("err: %s", err)
	}
	if *addr != DefaultHttpAddr {
		t.Fatalf("expect %q, got: %q", DefaultHttpAddr, *addr)
	}

	// Returns from the env var
	if err := os.Setenv(HttpEnvVar, "http://127.0.0.1:1111"); err != nil {
		t.Fatalf("err: %s", err)
	}
	flags = flag.NewFlagSet("test", flag.ContinueOnError)
	addr = httpAddrFlag(flags)
	if err := flags.Parse([]string{}); err != nil {
		t.Fatalf("err: %s", err)
	}
	if *addr != "http://127.0.0.1:1111" {
		t.Fatalf("expect %q, got: %q", "http://127.0.0.1:1111", *addr)
	}

	// Returns from flag
	flags = flag.NewFlagSet("test", flag.ContinueOnError)
	addr = httpAddrFlag(flags)
	if err := flags.Parse([]string{"-http-addr", "http://127.0.0.1:2222"}); err != nil {
		t.Fatalf("err: %s", err)
	}
	if *addr != "http://127.0.0.1:2222" {
		t.Fatalf("expect %q, got: %q", "http://127.0.0.1:2222", *addr)
	}
}
