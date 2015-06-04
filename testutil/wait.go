package testutil

import (
	"testing"
	"time"
)

type testFn func() (bool, error)
type errorFn func(error)

func WaitForResult(test testFn, error errorFn) {
	retries := 1000

	for retries > 0 {
		time.Sleep(10 * time.Millisecond)
		retries--

		success, err := test()
		if success {
			return
		}

		if retries == 0 {
			error(err)
		}
	}
}

type rpcFn func(string, interface{}, interface{}) error

func WaitForLeader(t *testing.T, rpc rpcFn) {
	WaitForResult(func() (bool, error) {
		args := struct{}{}
		var leader string
		err := rpc("Status.Leader", args, &leader)
		return leader != "", err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
}
