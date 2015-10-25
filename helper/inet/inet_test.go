package inet

import (
	"reflect"
	"testing"
)

func TestAdvertiseFromSubnet_CidrOutOfRange(t *testing.T) {
	if ad, err := AdvertiseIpFromSubnet("127.0.0.0/43:4648"); err == nil {
		t.Fatalf("expected advertise subnet error, got: %s", ad)
	}
}

func TestAdvertiseFromSubnet_IpSubnetOutOfRange(t *testing.T) {
	if ad, err := AdvertiseIpFromSubnet("333.0.0.0/8:4648"); err == nil {
		t.Fatalf("expected advertise subnet error, got: %s", ad)
	}
}

func TestAdvertiseFromSubnet_ShouldPassthrough(t *testing.T) {
	expected := "127.0.0.1:4648"
	ad, err := AdvertiseIpFromSubnet(expected)
	if err != nil {
		t.Fatalf("expected success, got: %s", err)
	}
	if !reflect.DeepEqual(ad, expected) {
		t.Fatalf("expected to get %s, got %s", expected, ad)
	}
}

func TestAdvertiseFromSubnet_ShouldPass(t *testing.T) {
	expected := "127.0.0.1:4648"
  ad, err := AdvertiseIpFromSubnet("127.0.0.0/8:4648")
	if err != nil {
		t.Fatalf("expected success, got: %s", err)
	}
	if !reflect.DeepEqual(ad, expected) {
		t.Fatalf("expected to get %s, got %s", expected, ad)
	}
}
