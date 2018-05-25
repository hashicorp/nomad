// +build !windows
package driver

import (
	"testing"
)

func TestCustomDriver_notExistingDriversDir(t *testing.T) {
	_, err := findCustomDrivers("notExistingDir")
	if err != nil {
		t.Error("not expected error - if dir does not exists", err)
	}
}

func TestCustomDriver_noDynamicLinkedDirs(t *testing.T) {
	files, err := findCustomDrivers("env" /*existing dir*/)
	if err != nil {
		t.Error("not expected error - when there is no custom driver", err)
	}
	if len(files) == 0 {
		t.Error("not expected file found - when there is no custom driver", err)
	}
}

func TestCustomDriver_foundNewDriver(t *testing.T) {
	err := loadCustomDrivers([]string{"plugin01"}, func(file string) (interface{}, error) {
		return NewRawExecDriver, nil
	})
	if err != nil {
		t.Error("not expected error - when there is found NewDriver()", err)
	}

	_, registered := BuiltinDrivers["plugin01"]
	if !registered {
		t.Error("expected plugin01 registration - when there is found NewDriver()", err)
	}
}

func TestCustomDriver_notfoundNewDriver(t *testing.T) {
	_ := loadCustomDrivers([]string{"plugin01"}, func(file string) (interface{}, error) {
		return nil, nil
	})

	_, registered := BuiltinDrivers["plugin01"]
	if registered {
		t.Error("not expected plugin01 registration - when there is found NewDriver()")
	}
}
