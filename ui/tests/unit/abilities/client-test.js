/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/avoid-leaking-state-in-ember-objects */
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Service from '@ember/service';
import setupAbility from 'nomad-ui/tests/helpers/setup-ability';

module('Unit | Ability | client', function (hooks) {
  setupTest(hooks);
  setupAbility('client')(hooks);

  test('it permits client read and write when ACLs are disabled', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: false,
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRead);
    assert.ok(this.ability.canWrite);
  });

  test('it permits client read and write for management tokens', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'management' },
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRead);
    assert.ok(this.ability.canWrite);
  });

  test('it permits client read and write for tokens with a policy that has node-write', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'write',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRead);
    assert.ok(this.ability.canWrite);
  });

  test('it permits client read and write for tokens with a policy that allows write and another policy that disallows it', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'write',
            },
          },
        },
        {
          rulesJSON: {
            Node: {
              Policy: 'read',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRead);
    assert.ok(this.ability.canWrite);
  });

  test('it permits client read and blocks client write for tokens with a policy that does not allow node-write', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {
            Node: {
              Policy: 'read',
            },
          },
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.ok(this.ability.canRead);
    assert.notOk(this.ability.canWrite);
  });

  test('it blocks client read and write for tokens without a node policy', function (assert) {
    const mockToken = Service.extend({
      aclEnabled: true,
      selfToken: { type: 'client' },
      selfTokenPolicies: [
        {
          rulesJSON: {},
        },
      ],
    });
    this.owner.register('service:token', mockToken);

    assert.notOk(this.ability.canRead);
    assert.notOk(this.ability.canWrite);
  });
});
