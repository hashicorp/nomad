// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build unix

package users

import (
	"errors"
	"os/user"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
	"oss.indeed.com/go/libtime/libtimetest"
)

func TestCache_real_hit(t *testing.T) {
	ci.Parallel(t)

	c := newCache()

	// fresh lookup
	u, err := c.GetUser("nobody")
	must.NoError(t, err)
	must.NotNil(t, u)

	// hit again, cached value
	u2, err2 := c.GetUser("nobody")
	must.NoError(t, err2)
	must.NotNil(t, u2)
	must.True(t, u == u2) // compare pointers
}

func TestCache_real_miss(t *testing.T) {
	ci.Parallel(t)

	c := newCache()

	// fresh lookup
	u, err := c.GetUser("doesnotexist")
	must.Error(t, err)
	must.Nil(t, u)

	// hit again, cached value
	u2, err2 := c.GetUser("doesnotexist")
	must.Error(t, err2)
	must.Nil(t, u2)
	must.True(t, err == err2) // compare pointers
}

func TestCache_mock_hit(t *testing.T) {
	ci.Parallel(t)

	c := newCache()

	lookupCount := 0

	// hijack the underlying lookup function with our own mock
	c.lookupUser = func(username string) (*user.User, error) {
		lookupCount++
		return &user.User{Name: username}, nil
	}

	// hijack the clock with our own mock
	t0 := time.Now()
	clockCount := 0
	c.clock = libtimetest.NewClockMock(t).NowMock.Set(func() time.Time {
		clockCount++
		switch clockCount {
		case 1:
			return t0
		case 2:
			return t0.Add(59 * time.Minute)
		default:
			return t0.Add(61 * time.Minute)
		}
	})

	const username = "armon"

	// initial lookup
	u, err := c.GetUser(username)
	must.NoError(t, err)
	must.Eq(t, "armon", u.Name)
	must.Eq(t, 1, lookupCount)
	must.Eq(t, 1, clockCount)

	// second lookup, 59 minutes after initil lookup
	u2, err2 := c.GetUser(username)
	must.NoError(t, err2)
	must.Eq(t, "armon", u2.Name)
	must.Eq(t, 1, lookupCount) // was in cache
	must.Eq(t, 2, clockCount)

	// third lookup, 61 minutes after initial lookup (expired)
	u3, err3 := c.GetUser(username)
	must.NoError(t, err3)
	must.Eq(t, "armon", u3.Name)
	must.Eq(t, 2, lookupCount)
	must.Eq(t, 3, clockCount)
}

func TestCache_mock_miss(t *testing.T) {
	ci.Parallel(t)

	c := newCache()

	lookupCount := 0
	lookupErr := errors.New("lookup error")

	// hijack the underlying lookup function with our own mock
	c.lookupUser = func(username string) (*user.User, error) {
		lookupCount++
		return nil, lookupErr
	}

	// hijack the clock with our own mock
	t0 := time.Now()
	clockCount := 0
	c.clock = libtimetest.NewClockMock(t).NowMock.Set(func() time.Time {
		clockCount++
		switch clockCount {
		case 1:
			return t0
		case 2:
			return t0.Add(59 * time.Second)
		default:
			return t0.Add(61 * time.Second)
		}
	})

	const username = "armon"

	// initial lookup
	u, err := c.GetUser(username)
	must.ErrorIs(t, err, lookupErr)
	must.Nil(t, u)
	must.Eq(t, 1, lookupCount)
	must.Eq(t, 1, clockCount)

	// second lookup, 59 seconds after initial (still in cache)
	u2, err2 := c.GetUser(username)
	must.ErrorIs(t, err2, lookupErr)
	must.Nil(t, u2)
	must.Eq(t, 1, lookupCount) // in cache
	must.Eq(t, 2, clockCount)

	// third lookup, 61 seconds after initial (expired)
	u3, err3 := c.GetUser(username)
	must.ErrorIs(t, err3, lookupErr)
	must.Nil(t, u3)
	must.Eq(t, 2, lookupCount)
	must.Eq(t, 3, clockCount)
}
