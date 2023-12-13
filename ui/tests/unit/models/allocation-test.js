/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | allocation', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
  });

  test("When the allocation's job version matches the job's version, the task group comes from the job.", function (assert) {
    const job = run(() =>
      this.store.createRecord('job', {
        name: 'this-job',
        version: 1,
        taskGroups: [
          {
            name: 'from-job',
            count: 1,
            task: [],
          },
        ],
      })
    );

    const allocation = run(() =>
      this.store.createRecord('allocation', {
        job,
        jobVersion: 1,
        taskGroupName: 'from-job',
        allocationTaskGroup: {
          name: 'from-allocation',
          count: 1,
          task: [],
        },
      })
    );

    assert.equal(allocation.get('taskGroup.name'), 'from-job');
  });

  test("When the allocation's job version does not match the job's version, the task group comes from the alloc.", function (assert) {
    const job = run(() =>
      this.store.createRecord('job', {
        name: 'this-job',
        version: 1,
        taskGroups: [
          {
            name: 'from-job',
            count: 1,
            task: [],
          },
        ],
      })
    );

    const allocation = run(() =>
      this.store.createRecord('allocation', {
        job,
        jobVersion: 2,
        taskGroupName: 'from-job',
        allocationTaskGroup: {
          name: 'from-allocation',
          count: 1,
          task: [],
        },
      })
    );

    assert.equal(allocation.get('taskGroup.name'), 'from-allocation');
  });

  test("When the allocation's job version does not match the job's version and the allocation has no task group, then task group is null", async function (assert) {
    const job = run(() =>
      this.store.createRecord('job', {
        name: 'this-job',
        version: 1,
        taskGroups: [
          {
            name: 'from-job',
            count: 1,
            task: [],
          },
        ],
      })
    );

    const allocation = run(() =>
      this.store.createRecord('allocation', {
        job,
        jobVersion: 2,
        taskGroupName: 'from-job',
      })
    );

    assert.equal(allocation.get('taskGroup.name'), null);
  });
});
