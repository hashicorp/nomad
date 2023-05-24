/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
    const nodePool = this.store.createRecord('node-pool', {
      name: 'prod-eng',
      description: 'Production workloads',
      meta: {
        env: 'production',
        team: 'engineering',
      },
      schedulerConfiguration: {
        SchedulerAlgorithm: 'spread',
      },
    });

    const serializedNodePool = this.subject().serialize(
      nodePool._createSnapshot()
    );

    assert.deepEqual(serializedNodePool, {
      Name: 'prod-eng',
      Description: 'Production workloads',
      Meta: {
        env: 'production',
        team: 'engineering',
      },
      SchedulerConfiguration: {
        SchedulerAlgorithm: 'spread',
      },
    });
  });
});
