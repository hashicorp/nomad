package main

import (
	"testing"
)

func TestGenSchema(t *testing.T) {
	err := genSchema()
	if err != nil {
		t.Errorf("TestGenSchema failed with error: %+v", err)
	}
}

func TestLoadPackages(t *testing.T) {
	err := loadPackages()
	if err != nil {
		t.Errorf("TestLoadPackages failed with error: %+v", err)
	}
}