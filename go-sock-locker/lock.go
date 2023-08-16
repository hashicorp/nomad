package main

import (
	"context"
	"log"

	"github.com/hashicorp/nomad/api"
)

func init() {
}

// TODO: modify this interface?  i'd prefer to pass lockID into Release/Renew
type lock interface {
	Acquire(ctx context.Context, callerID string) (lockID string, err error)
	Release(ctx context.Context) error
	Renew(ctx context.Context) error
}

var _ lock = &Lock{}

type Lock struct {
	client *api.Client
	// TODO: logger
	//job string

	id     string // protect me?
	caller string // why?
}

func (l *Lock) path() string {
	//return l.job + "/lock"
	return lockPath
}

func (l *Lock) varReq(ctx context.Context) (*api.Variable, *api.WriteOptions) {
	o := &api.WriteOptions{}
	o.WithContext(ctx)
	v := &api.Variable{
		Path: l.path(),
		//Items: map[string]string{
		//	"caller": l.caller,
		//},
		Lock: &api.VariableLock{
			TTL:       "10s",
			LockDelay: "10s",
		},
	}
	return v, o
}

func (l *Lock) Acquire(ctx context.Context, callerID string) (string, error) {
	l.caller = callerID
	v, o := l.varReq(ctx)
	v.Items = map[string]string{
		//"caller": l.caller,
		"alloc": allocID,
	}

	vv, _, err := l.client.Variables().AcquireLock(v, o)
	if err != nil {
		return "", err
		//return "", fmt.Errorf("failed to acquire lock: %w", err)
	}
	log.Printf("acquired! %+v", vv.Lock)

	l.id = vv.Lock.ID
	return vv.Lock.ID, nil
}

func (l *Lock) Release(ctx context.Context) error {
	v, o := l.varReq(ctx)
	v.Lock.ID = l.id

	vv, _, err := l.client.Variables().ReleaseLock(v, o)
	if err != nil {
		return err
		//return fmt.Errorf("failed to release lock: %w", err)
	}
	log.Printf("released! %+v", vv.Lock)

	l.id = ""
	l.caller = ""
	return nil
}

func (l *Lock) Renew(ctx context.Context) error {
	v, o := l.varReq(ctx)
	v.Lock.ID = l.id
	v.Items = map[string]string{
		//"caller": l.caller,
		"alloc": allocID,
		//"renewed": "true",
	}

	vv, _, err := l.client.Variables().RenewLock(v, o)
	if err != nil {
		return err
		//return fmt.Errorf("failed to renew lock: %w", err)
	}
	log.Printf("renewed! %+v", vv.Lock)

	return nil
}
