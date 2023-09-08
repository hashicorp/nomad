/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | allocation', function (hooks) {
  setupTest(hooks);
  setupAbility('allocation')(hooks);

  test('it permits alloc exec when ACLs are disabled', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.can.can('exec allocation'));
  });

  test('it permits alloc exec for management tokens', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.can.can('exec allocation'));
  });

  test('it permits alloc exec for client tokens with a policy that has namespace alloc-exec', function (assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Namespaces: [
              {
                Name: 'aNamespace',
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(
      this.can.can('exec allocation', null, { namespace: 'aNamespace' })
    );
  });

  test('it permits alloc exec for client tokens with a policy that has default namespace alloc-exec and no capabilities for active namespace', function (assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Namespaces: [
              {
                Name: 'aNamespace',
                Capabilities: [],
              },
              {
                Name: 'default',
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(
      this.can.can('exec allocation', null, { namespace: 'anotherNamespace' })
    );
  });

  test('it blocks alloc exec for client tokens with a policy that has no alloc-exec capability', function (assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Namespaces: [
              {
                Name: 'aNamespace',
                Capabilities: ['list-jobs'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(
      this.can.cannot('exec allocation', null, { namespace: 'aNamespace' })
    );
  });

  test('it handles globs in namespace names', function (assert) {
    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Namespaces: [
              {
                Name: 'production-*',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: 'production-api',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: 'production-web',
                Capabilities: [],
              },
              {
                Name: '*-suffixed',
                Capabilities: ['alloc-exec'],
              },
              {
                Name: '*-more-suffixed',
                Capabilities: [],
              },
              {
                Name: '*-abc-*',
                Capabilities: ['alloc-exec'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(
      this.can.cannot('exec allocation', null, { namespace: 'production-web' })
    );
    assert.ok(
      this.can.can('exec allocation', null, { namespace: 'production-api' })
    );
    assert.ok(
      this.can.can('exec allocation', null, { namespace: 'production-other' })
    );
    assert.ok(
      this.can.can('exec allocation', null, { namespace: 'something-suffixed' })
    );
    assert.ok(
      this.can.cannot('exec allocation', null, {
        namespace: 'something-more-suffixed',
      }),
      'expected the namespace with the greatest number of matched characters to be chosen'
    );
    assert.ok(
      this.can.can('exec allocation', null, { namespace: '000-abc-999' }),
      'expected to be able to match against more than one wildcard'
    );
  });
});
