/**
 * Copyright IBM Corp. 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import {
  allocationMatchesGroupSelections,
  groupSelectionAllocationCount,
  groupSelectionStatuses,
} from 'nomad-ui/utils/group-selection-status';

const job = (count = 1) => ({
  version: 2,
  createIndex: 100,
  groupSelections: [
    { Name: 'runtime', Count: count, Groups: ['encoder', 'orin', 'thor'] },
  ],
  taskGroups: [
    { name: 'encoder', count: 4 },
    { name: 'orin', count: 2 },
    { name: 'thor', count: 1 },
    { name: 'control', count: 2 },
  ],
});

const allocation = (group, cohort, values = {}) => ({
  taskGroupName: group,
  groupSelection: { Name: 'runtime', Slot: 0, Cohort: cohort },
  clientStatus: 'running',
  desiredStatus: 'run',
  jobVersion: 2,
  createIndex: 101,
  ...values,
});

module('Unit | Utility | group selection status', function () {
  test('selected groups contribute their own replica counts', function (assert) {
    const current = allocation('thor', 'current');
    const stale = allocation('encoder', 'old', { clientStatus: 'failed' });
    const statuses = groupSelectionStatuses(job(), [stale, current]);

    assert.strictEqual(
      groupSelectionAllocationCount(job().taskGroups, statuses),
      3,
    );
    assert.strictEqual(statuses.runtime.Unplaced, 0);
    assert.true(allocationMatchesGroupSelections(current, statuses));
    assert.false(allocationMatchesGroupSelections(stale, statuses));
    assert.true(
      allocationMatchesGroupSelections({ taskGroupName: 'control' }, statuses),
    );
  });

  test('unresolved choices are separate from allocation demand', function (assert) {
    const current = allocation('thor', 'current');
    const statuses = groupSelectionStatuses(job(2), [current]);
    assert.strictEqual(statuses.runtime.Unplaced, 1);
    assert.strictEqual(
      groupSelectionAllocationCount(job(2).taskGroups, statuses),
      3,
    );

    const empty = groupSelectionStatuses(job(), []);
    assert.strictEqual(empty.runtime.Unplaced, 1);
    assert.strictEqual(
      groupSelectionAllocationCount(job().taskGroups, empty),
      2,
    );
  });

  test('active deployments use their target cohort', function (assert) {
    const old = allocation('encoder', 'old', { jobVersion: 1 });
    const failed = allocation('orin', 'rejected', {
      clientStatus: 'failed',
      isCanary: true,
    });
    const canary = allocation('thor', 'target', { isCanary: true });
    const deployment = {
      versionNumber: 2,
      status: 'running',
      groupSelections: {
        runtime: {
          Count: 1,
          Slots: {
            0: {
              TaskGroup: 'thor',
              Cohort: 'target',
              PreviousTaskGroup: 'encoder',
              PreviousCohort: 'old',
            },
          },
        },
      },
    };
    const statuses = groupSelectionStatuses(
      job(),
      [old, failed, canary],
      deployment,
    );
    assert.strictEqual(
      groupSelectionAllocationCount(job().taskGroups, statuses),
      3,
    );
    assert.true(allocationMatchesGroupSelections(canary, statuses));
    assert.false(allocationMatchesGroupSelections(old, statuses));
    assert.false(allocationMatchesGroupSelections(failed, statuses));
  });

  test('failed accepted cohorts retain concrete demand without becoming healthy', function (assert) {
    const failed = allocation('encoder', 'failed', { clientStatus: 'failed' });
    const statuses = groupSelectionStatuses(job(), [failed]);
    assert.strictEqual(statuses.runtime.Slots[0].TaskGroup, 'encoder');
    assert.strictEqual(
      groupSelectionAllocationCount(job().taskGroups, statuses),
      6,
    );
    assert.strictEqual(failed.clientStatus, 'failed');
  });

  test('previous job incarnations and stopped allocations do not select groups', function (assert) {
    const historical = allocation('encoder', 'historical', { createIndex: 50 });
    const stopped = allocation('thor', 'stopped', { desiredStatus: 'stop' });
    const statuses = groupSelectionStatuses(job(), [historical, stopped]);
    assert.strictEqual(statuses.runtime.Unplaced, 1);
    assert.deepEqual(statuses.runtime.Slots, {});
  });
});
