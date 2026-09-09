/**
 * Copyright IBM Corp. 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | job group selection', function (hooks) {
  setupTest(hooks);

  test('job details derive selections from allocation relationships', function (assert) {
    const store = this.owner.lookup('service:store');
    const current = store.createRecord('allocation', {
      id: 'detail-current',
      taskGroupName: 'thor',
      clientStatus: 'running',
      desiredStatus: 'run',
      groupSelection: { Name: 'runtime', Slot: 0, Cohort: 'current' },
    });
    const job = store.createRecord('job', {
      type: 'service',
      status: 'running',
      groupSelections: [
        { Name: 'runtime', Count: 1, Groups: ['encoder', 'orin', 'thor'] },
      ],
      taskGroups: [
        { name: 'encoder', count: 4 },
        { name: 'orin', count: 2 },
        { name: 'thor', count: 1 },
      ],
      allocations: [current],
    });

    assert.strictEqual(job.effectiveGroupSelectionStatuses.runtime.Unplaced, 0);
    assert.strictEqual(job.expectedRunningAllocCount, 1);
    assert.deepEqual(job.aggregateAllocStatus, {
      label: 'Healthy',
      state: 'success',
    });
  });

  test('one selected group is healthy without filling inactive alternatives', function (assert) {
    const store = this.owner.lookup('service:store');
    const current = store.createRecord('allocation', {
      id: 'current',
      taskGroupName: 'thor',
      clientStatus: 'running',
      desiredStatus: 'run',
      groupSelection: { Name: 'runtime', Slot: 0, Cohort: 'current' },
    });
    const previous = store.createRecord('allocation', {
      id: 'previous',
      taskGroupName: 'encoder',
      clientStatus: 'running',
      desiredStatus: 'stop',
      groupSelection: { Name: 'runtime', Slot: 0, Cohort: 'previous' },
    });
    const job = store.createRecord('job', {
      type: 'service',
      status: 'running',
      groupCountSum: 1,
      groupSelectionStatuses: {
        runtime: {
          Count: 1,
          Groups: ['encoder', 'orin', 'thor'],
          Slots: { 0: { TaskGroup: 'thor', Cohort: 'current' } },
          Unplaced: 0,
        },
      },
      allocations: [previous, current],
    });
    assert.deepEqual(job.aggregateAllocStatus, {
      label: 'Healthy',
      state: 'success',
    });
    assert.deepEqual(job.allocBlocks.running.healthy.nonCanary, [current]);
    assert.strictEqual(job.allocBlocks.unplaced.healthy.nonCanary.length, 0);
  });

  test('unfilled selection slots degrade the job even with no concrete allocation demand', function (assert) {
    const store = this.owner.lookup('service:store');
    const job = store.createRecord('job', {
      type: 'service',
      status: 'pending',
      groupCountSum: 0,
      groupSelectionStatuses: {
        runtime: {
          Count: 1,
          Groups: ['encoder', 'orin', 'thor'],
          Slots: {},
          Unplaced: 1,
        },
      },
    });
    assert.deepEqual(job.aggregateAllocStatus, {
      label: 'Degraded',
      state: 'warning',
    });
    assert.strictEqual(job.expectedRunningAllocCount, 0);
    assert.strictEqual(job.unplacedGroupSelections, 1);
  });
});
