package common

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestReadlines(t *testing.T) {
	ret, err := ReadLines("common_test.go")
	if err != nil {
		t.Error(err)
	}
	if !strings.Contains(ret[0], "package common") {
		t.Error("could not read correctly")
	}
}

func TestReadLinesOffsetN(t *testing.T) {
	ret, err := ReadLinesOffsetN("common_test.go", 2, 1)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(ret[0])
	if !strings.Contains(ret[0], `import (`) {
		t.Error("could not read correctly")
	}
}

func TestIntToString(t *testing.T) {
	src := []int8{65, 66, 67}
	dst := IntToString(src)
	if dst != "ABC" {
		t.Error("could not convert")
	}
}
func TestByteToString(t *testing.T) {
	src := []byte{65, 66, 67}
	dst := ByteToString(src)
	if dst != "ABC" {
		t.Error("could not convert")
	}

	src = []byte{0, 65, 66, 67}
	dst = ByteToString(src)
	if dst != "ABC" {
		t.Error("could not convert")
	}
}

func TestmustParseInt32(t *testing.T) {
	ret := mustParseInt32("11111")
	if ret != int32(11111) {
		t.Error("could not parse")
	}
}
func TestmustParseUint64(t *testing.T) {
	ret := mustParseUint64("11111")
	if ret != uint64(11111) {
		t.Error("could not parse")
	}
}
func TestmustParseFloat64(t *testing.T) {
	ret := mustParseFloat64("11111.11")
	if ret != float64(11111.11) {
		t.Error("could not parse")
	}
	ret = mustParseFloat64("11111")
	if ret != float64(11111) {
		t.Error("could not parse")
	}
}
func TestStringsContains(t *testing.T) {
	target, err := ReadLines("common_test.go")
	if err != nil {
		t.Error(err)
	}
	if !StringsContains(target, "func TestStringsContains(t *testing.T) {") {
		t.Error("cloud not test correctly")
	}
}

func TestPathExists(t *testing.T) {
	if !PathExists("common_test.go") {
		t.Error("exists but return not exists")
	}
	if PathExists("should_not_exists.go") {
		t.Error("not exists but return exists")
	}
}

func TestHostEtc(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows doesn't have etc")
	}
	p := HostEtc("mtab")
	if p != "/etc/mtab" {
		t.Errorf("invalid HostEtc, %s", p)
	}
}

func TestGetSysctrlEnv(t *testing.T) {
	// Append case
	env := getSysctrlEnv([]string{"FOO=bar"})
	if !reflect.DeepEqual(env, []string{"FOO=bar", "LC_ALL=C"}) {
		t.Errorf("unexpected append result from getSysctrlEnv: %q", env)
	}

	// Replace case
	env = getSysctrlEnv([]string{"FOO=bar", "LC_ALL=en_US.UTF-8"})
	if !reflect.DeepEqual(env, []string{"FOO=bar", "LC_ALL=C"}) {
		t.Errorf("unexpected replace result from getSysctrlEnv: %q", env)
	}

	// Test against real env
	env = getSysctrlEnv(os.Environ())
	found := false
	for _, v := range env {
		if v == "LC_ALL=C" {
			found = true
			continue
		}
		if strings.HasPrefix(v, "LC_ALL") {
			t.Fatalf("unexpected LC_ALL value: %q", v)
		}
	}
	if !found {
		t.Errorf("unexpected real result from getSysctrlEnv: %q", env)
	}
}
