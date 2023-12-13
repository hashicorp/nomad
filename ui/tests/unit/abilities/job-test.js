/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | job', function (hooks) {
  setupTest(hooks);
  setupAbility('job')(hooks);

  test('it permits job run when ACLs are disabled', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it permits job run for management tokens', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });

    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRun);
  });

  test('it permits job run for client tokens with a policy that has namespace submit-job', function (assert) {
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
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.can.can('run job', null, { namespace: 'aNamespace' }));
  });

  test('it permits job run for client tokens with a policy that has default namespace submit-job and no capabilities for active namespace', function (assert) {
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
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(this.can.can('run job', null, { namespace: 'anotherNamespace' }));
  });

  test('it blocks job run for client tokens with a policy that has no submit-job capability', function (assert) {
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

    assert.ok(this.can.cannot('run job', null, { namespace: 'aNamespace' }));
  });

  test('job scale requires a client token with the submit-job or scale-job capability', function (assert) {
    const makePolicies = (namespace, ...capabilities) => [
      {
        rulesJSON: {
          Namespaces: [
            {
              Name: namespace,
              Capabilities: capabilities,
            },
          ],
        },
      },
    ];

    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: makePolicies('aNamespace'),
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);
    const tokenService = this.owner.lookup('service:token');

    assert.ok(this.can.cannot('scale job', null, { namespace: 'aNamespace' }));

    tokenService.set(
      'selfTokenPolicies',
      makePolicies('aNamespace', 'scale-job')
    );
    assert.ok(this.can.can('scale job', null, { namespace: 'aNamespace' }));

    tokenService.set(
      'selfTokenPolicies',
      makePolicies('aNamespace', 'submit-job')
    );
    assert.ok(this.can.can('scale job', null, { namespace: 'aNamespace' }));

    tokenService.set(
      'selfTokenPolicies',
      makePolicies('bNamespace', 'scale-job')
    );
    assert.ok(this.can.cannot('scale job', null, { namespace: 'aNamespace' }));
  });

  test('job dispatch requires a client token with the dispatch-job capability', function (assert) {
    const makePolicies = (namespace, ...capabilities) => [
      {
        rulesJSON: {
          Namespaces: [
            {
              Name: namespace,
              Capabilities: capabilities,
            },
          ],
        },
      },
    ];

    const mockSystem = Service.extend({
      aclEnabled: true,
    });

    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: makePolicies('aNamespace'),
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);
    const tokenService = this.owner.lookup('service:token');

    assert.ok(
      this.can.cannot('dispatch job', null, { namespace: 'aNamespace' })
    );

    tokenService.set(
      'selfTokenPolicies',
      makePolicies('aNamespace', 'dispatch-job')
    );
    assert.ok(this.can.can('dispatch job', null, { namespace: 'aNamespace' }));
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
                Capabilities: ['submit-job'],
              },
              {
                Name: 'production-api',
                Capabilities: ['submit-job'],
              },
              {
                Name: 'production-web',
                Capabilities: [],
              },
              {
                Name: '*-suffixed',
                Capabilities: ['submit-job'],
              },
              {
                Name: '*-more-suffixed',
                Capabilities: [],
              },
              {
                Name: '*-abc-*',
                Capabilities: ['submit-job'],
              },
            ],
          },
        },
      ],
    });

    this.owner.register('service:system', mockSystem);
    this.owner.register('service:token', mockToken);

    assert.ok(
      this.can.can(
        'run job',
        null,
        { namespace: 'production-web' },
        'The existence of a single namespace where a job can be run means that can run is enabled'
      )
    );
    assert.ok(this.can.can('run job', null, { namespace: 'production-api' }));
    assert.ok(this.can.can('run job', null, { namespace: 'production-other' }));
    assert.ok(
      this.can.can('run job', null, { namespace: 'something-suffixed' })
    );
    assert.ok(
      this.can.can('run job', null, { namespace: '000-abc-999' }),
      'expected to be able to match against more than one wildcard'
    );
  });
});
