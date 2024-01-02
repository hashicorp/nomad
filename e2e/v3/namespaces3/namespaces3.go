// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package namespaces3

import (
	"testing"
	"time"

	"github.com/hashicorp/go-set"
	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/v3/util3"
	"github.com/hashicorp/nomad/helper"
	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
)

type Names struct {
	t *testing.T

	nomadClient *nomadapi.Client

	noCleanup bool
	timeout   time.Duration
	verbose   bool

	apply  *set.HashSet[*Namespace, string]
	remove *set.Set[string]
}

func (g *Names) logf(msg string, args ...any) {
	util3.Log3(g.t, g.verbose, msg, args...)
}

func (g *Names) cleanup() {
	if g.noCleanup {
		return
	}

	namespaceAPI := g.nomadClient.Namespaces()

	// remove any namespaces we created (or updated)
	for _, namespace := range g.apply.Slice() {
		name := namespace.Name
		g.logf("cleanup namespace %q", name)
		_, err := namespaceAPI.Delete(name, nil)
		test.NoError(g.t, err, test.Sprintf("unable to delete namespace %q", name))
	}
}

type Option func(*Names)

type Cleanup func()

type Namespace struct {
	Name        string
	Description string
}

func (ns *Namespace) Hash() string {
	return ns.Name
}

func (ns *Namespace) String() string {
	return ns.Name
}

func (n *Names) setClient() {
	nomadClient, nomadErr := nomadapi.NewClient(nomadapi.DefaultConfig())
	must.NoError(n.t, nomadErr, must.Sprint("failed to create nomad api client"))
	n.nomadClient = nomadClient
}

func configure(t *testing.T, opts ...Option) Cleanup {
	g := &Names{
		t:       t,
		timeout: 10 * time.Second,
		apply:   set.NewHashSet[*Namespace, string](3),
		remove:  set.New[string](3),
	}

	for _, opt := range opts {
		opt(g)
	}

	g.setClient()
	g.run()

	return g.cleanup
}

func (g *Names) run() {
	namespacesAPI := g.nomadClient.Namespaces()

	// do deletions
	for _, namespace := range g.remove.Slice() {
		g.logf("delete namespace %q", namespace)
		_, err := namespacesAPI.Delete(namespace, nil)
		must.NoError(g.t, err)
	}

	// do applies
	for _, namespace := range g.apply.Slice() {
		g.logf("apply namespace %q", namespace)
		_, err := namespacesAPI.Register(&nomadapi.Namespace{
			Name:        namespace.Name,
			Description: namespace.Description,
		}, nil)
		must.NoError(g.t, err)
	}
}

// Create a namespace of the given name.
func Create(t *testing.T, name string, opts ...Option) Cleanup {
	namespace := &Namespace{Name: name}
	opt := apply(namespace)
	return configure(t, append(opts, opt)...)
}

// Create namespaces of the given names.
func CreateN(t *testing.T, names []string, opts ...Option) Cleanup {
	creations := helper.ConvertSlice(names, func(name string) Option {
		namespace := &Namespace{Name: name}
		return apply(namespace)
	})
	return configure(t, append(opts, creations...)...)
}

// Delete the namespace of the given name.
func Delete(t *testing.T, name string, opts ...Option) Cleanup {
	opt := remove(name)
	return configure(t, append(opts, opt)...)
}

func apply(namespace *Namespace) Option {
	return func(g *Names) {
		g.apply.Insert(namespace)
	}
}

func remove(name string) Option {
	return func(g *Names) {
		g.remove.Insert(name)
	}
}

// DisableCleanup will disable the automatic removal of any namespaces
// created using the Create() Option.
func DisableCleanup() Option {
	return func(n *Names) {
		n.noCleanup = true
	}
}

func Timeout(timeout time.Duration) Option {
	return func(n *Names) {
		n.timeout = timeout
	}
}

// Verbose will enable verbose logging.
func Verbose(on bool) Option {
	return func(n *Names) {
		n.verbose = on
	}
}
