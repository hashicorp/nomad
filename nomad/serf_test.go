package nomad

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/hashicorp/nomad/testutil"
)

func TestNomad_JoinPeer(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()
	s2Addr := fmt.Sprintf("127.0.0.1:%d", s2.config.SerfConfig.MemberlistConfig.BindPort)

	num, err := s1.Join([]string{s2Addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 1 {
		t.Fatalf("bad: %d", num)
	}

	testutil.WaitForResult(func() (bool, error) {
		if members := s1.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s2.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	testutil.WaitForResult(func() (bool, error) {
		if len(s1.peers) != 2 {
			return false, fmt.Errorf("bad: %#v", s1.peers)
		}
		if len(s2.peers) != 2 {
			return false, fmt.Errorf("bad: %#v", s2.peers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestNomad_RemovePeer(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.Region = "region2"
	})
	defer s2.Shutdown()
	s2Addr := fmt.Sprintf("127.0.0.1:%d", s2.config.SerfConfig.MemberlistConfig.BindPort)

	num, err := s1.Join([]string{s2Addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 1 {
		t.Fatalf("bad: %d", num)
	}

	testutil.WaitForResult(func() (bool, error) {
		if members := s1.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		if members := s2.Members(); len(members) != 2 {
			return false, fmt.Errorf("bad: %#v", members)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	// Leave immediately
	s2.Leave()
	s2.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		if len(s1.peers) != 1 {
			return false, fmt.Errorf("bad: %#v", s1.peers)
		}
		if len(s2.peers) != 1 {
			return false, fmt.Errorf("bad: %#v", s2.peers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}

func TestNomad_BootstrapExpect(t *testing.T) {
	dir := tmpDir(t)
	defer os.RemoveAll(dir)

	s1 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node1")
	})
	defer s1.Shutdown()
	s2 := testServer(t, func(c *Config) {
		c.BootstrapExpect = 2
		c.DevMode = false
		c.DataDir = path.Join(dir, "node2")
	})
	defer s2.Shutdown()
	s2Addr := fmt.Sprintf("127.0.0.1:%d", s2.config.SerfConfig.MemberlistConfig.BindPort)

	num, err := s1.Join([]string{s2Addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if num != 1 {
		t.Fatalf("bad: %d", num)
	}

	testutil.WaitForResult(func() (bool, error) {
		peers, err := s1.numOtherPeers()
		if err != nil {
			return false, err
		}
		if peers != 1 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		peers, err = s2.numOtherPeers()
		if err != nil {
			return false, err
		}
		if peers != 1 {
			return false, fmt.Errorf("bad: %#v", peers)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})
}
