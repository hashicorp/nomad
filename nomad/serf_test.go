package nomad

import (
	"fmt"
	"testing"
)

func TestNomad_JoinPeer(t *testing.T) {
	s1 := testServer(t, nil)
	s2 := testServer(t, nil)
	s2Addr := fmt.Sprintf("127.0.0.1:%d", s2.config.SerfConfig.MemberlistConfig.BindPort)

	num, err := s1.Join([]string{s2Addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 1 {
		t.Fatalf("bad: %d", num)
	}

	if members := s1.Members(); len(members) != 2 {
		t.Fatalf("bad: %#v", members)
	}
	if members := s2.Members(); len(members) != 2 {
		t.Fatalf("bad: %#v", members)
	}
}
