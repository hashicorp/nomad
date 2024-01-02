/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { run } from '@ember/runloop';

const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

module('Unit | Model | task-group', function (hooks) {
  setupTest(hooks);

  test("should expose reserved resource stats as aggregates of each task's reserved resources", function (assert) {
    const taskGroup = run(() =>
      this.owner.lookup('service:store').createRecord('task-group', {
        name: 'group-example',
        tasks: [
          {
            name: 'task-one',
            driver: 'docker',
            reservedMemory: 512,
            reservedCPU: 500,
            reservedDisk: 1024,
          },
          {
            name: 'task-two',
            driver: 'docker',
            reservedMemory: 256,
            reservedCPU: 1000,
            reservedDisk: 512,
          },
          {
            name: 'task-three',
            driver: 'docker',
            reservedMemory: 1024,
            reservedCPU: 1500,
            reservedDisk: 4096,
          },
          {
            name: 'task-four',
            driver: 'docker',
            reservedMemory: 2048,
            reservedCPU: 500,
            reservedDisk: 128,
          },
        ],
      })
    );

    assert.equal(
      taskGroup.get('reservedCPU'),
      sum(taskGroup.get('tasks'), 'reservedCPU'),
      'reservedCPU is an aggregate sum of task CPU reservations'
    );
    assert.equal(
      taskGroup.get('reservedMemory'),
      sum(taskGroup.get('tasks'), 'reservedMemory'),
      'reservedMemory is an aggregate sum of task memory reservations'
    );
    assert.equal(
      taskGroup.get('reservedDisk'),
      sum(taskGroup.get('tasks'), 'reservedDisk'),
      'reservedDisk is an aggregate sum of task disk reservations'
    );
  });

  test("should expose mergedMeta as merged with the job's meta", function (assert) {
    assert.expect(8);

    const store = this.owner.lookup('service:store');

    const jobWithMeta = run(() =>
      store.createRecord('job', {
        name: 'example-with-meta',
        meta: store.createFragment('structured-attributes', {
          raw: { a: 'b' },
        }),
        taskGroups: [
          {
            name: 'one',
            meta: { c: 'd' },
          },
          {
            name: 'two',
          },
          {
            name: 'three',
            meta: null,
          },
          {
            name: 'four',
            meta: {},
          },
        ],
      })
    );

    let expected = [{ a: 'b', c: 'd' }, { a: 'b' }, { a: 'b' }, { a: 'b' }];
    expected.forEach((exp, i) => {
      assert.deepEqual(
        jobWithMeta.get('taskGroups').objectAt(i).get('mergedMeta'),
        exp,
        'mergedMeta is merged with job meta'
      );
    });

    const jobWithoutMeta = run(() =>
      this.owner.lookup('service:store').createRecord('job', {
        name: 'example-without-meta',
        taskGroups: [
          {
            name: 'one',
            meta: { c: 'd' },
          },
          {
            name: 'two',
          },
          {
            name: 'three',
            meta: null,
          },
          {
            name: 'four',
            meta: {},
          },
        ],
      })
    );

    expected = [{ c: 'd' }, {}, {}, {}];
    expected.forEach((exp, i) => {
      assert.deepEqual(
        jobWithoutMeta.get('taskGroups').objectAt(i).get('mergedMeta'),
        exp,
        'mergedMeta is merged with job meta'
      );
    });
  });
});
