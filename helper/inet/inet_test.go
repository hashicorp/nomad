package inet

import (
	"reflect"
	"testing"
)

func TestAdvertiseFromSubnet(t *testing.T) {
	// CIDR is out of range
	if ad, err := AdvertiseIpFromSubnet("127.0.0.0/43:4648"); err == nil {
		t.Fatalf("expected advertise subnet error, got: %s", ad)
	}

	// IP subnet is out of range
	if ad, err := AdvertiseIpFromSubnet("333.0.0.0/8:4648"); err == nil {
		t.Fatalf("expected advertise subnet error, got: %s", ad)
	}

	// Regular unicast IP definition: should return untouched string
	expected := "127.0.0.1:4648"
	ad, err := AdvertiseIpFromSubnet(expected)
	if err != nil {
		t.Fatalf("expected success, got: %s", err)
	}
	if !reflect.DeepEqual(ad, expected) {
		t.Fatalf("expected to get %s, got %s", expected, ad)
	}

	// Actual subnet definition: should succeed
	ad, err = AdvertiseIpFromSubnet("127.0.0.0/8:4648")
	if err != nil {
		t.Fatalf("expected success, got: %s", err)
	}
	if !reflect.DeepEqual(ad, expected) {
		t.Fatalf("expected to get %s, got %s", expected, ad)
	}
}
