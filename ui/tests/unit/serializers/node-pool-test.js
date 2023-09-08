/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { setupTest } from 'ember-qunit';
import { module, test } from 'qunit';

module('Unit | Serializer | NodePool', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('node-pool');
  });

  test('should serialize a NodePool', function (assert) {
    const testCases = [
      {
        name: 'full node pool',
        input: {
          name: 'prod-eng',
          description: 'Production workloads',
          meta: {
            env: 'production',
            team: 'engineering',
          },
          schedulerConfiguration: {
            SchedulerAlgorithm: 'spread',
            MemoryOversubscriptionEnabled: true,
          },
        },
        expected: {
          Name: 'prod-eng',
          Description: 'Production workloads',
          Meta: {
            env: 'production',
            team: 'engineering',
          },
          SchedulerConfiguration: {
            SchedulerAlgorithm: 'spread',
            MemoryOversubscriptionEnabled: true,
          },
        },
      },
      {
        name: 'node pool without scheduler configuration',
        input: {
          name: 'prod-eng',
          description: 'Production workloads',
          meta: {
            env: 'production',
            team: 'engineering',
          },
        },
        expected: {
          Name: 'prod-eng',
          Description: 'Production workloads',
          Meta: {
            env: 'production',
            team: 'engineering',
          },
          SchedulerConfiguration: undefined,
        },
      },
      {
        name: 'node pool with null scheduler configuration',
        input: {
          name: 'prod-eng',
          description: 'Production workloads',
          meta: {
            env: 'production',
            team: 'engineering',
          },
          schedulerConfiguration: null,
        },
        expected: {
          Name: 'prod-eng',
          Description: 'Production workloads',
          Meta: {
            env: 'production',
            team: 'engineering',
          },
          SchedulerConfiguration: null,
        },
      },
      {
        name: 'node pool with empty scheduler configuration',
        input: {
          name: 'prod-eng',
          description: 'Production workloads',
          meta: {
            env: 'production',
            team: 'engineering',
          },
          schedulerConfiguration: {},
        },
        expected: {
          Name: 'prod-eng',
          Description: 'Production workloads',
          Meta: {
            env: 'production',
            team: 'engineering',
          },
          SchedulerConfiguration: {},
        },
      },
    ];

    assert.expect(testCases.length);
    for (const tc of testCases) {
      const nodePool = this.store.createRecord('node-pool', tc.input);
      const got = this.subject().serialize(nodePool._createSnapshot());
      assert.deepEqual(
        got,
        tc.expected,
        `${tc.name} failed, got ${JSON.stringify(got)}`
      );
    }
  });
});
