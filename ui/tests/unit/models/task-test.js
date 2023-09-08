/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { run } from '@ember/runloop';

module('Unit | Model | task', function (hooks) {
  setupTest(hooks);

  test("should expose mergedMeta as merged with the job's and task groups's meta", function (assert) {
    assert.expect(8);

    const job = run(() =>
      this.owner.lookup('service:store').createRecord('job', {
        name: 'example',
        taskGroups: [
          {
            name: 'one',
            meta: { a: 'b' },
            tasks: [
              {
                name: 'task-one',
                meta: { c: 'd' },
              },
              {
                name: 'task-two',
              },
              {
                name: 'task-three',
                meta: null,
              },
              {
                name: 'task-four',
                meta: {},
              },
            ],
          },
          {
            name: 'two',
            tasks: [
              {
                name: 'task-one',
                meta: { c: 'd' },
              },
              {
                name: 'task-two',
              },
              {
                name: 'task-three',
                meta: null,
              },
              {
                name: 'task-four',
                meta: {},
              },
            ],
          },
        ],
      })
    );

    let tg = job.get('taskGroups').objectAt(0);
    let expected = [{ a: 'b', c: 'd' }, { a: 'b' }, { a: 'b' }, { a: 'b' }];

    expected.forEach((exp, i) => {
      assert.deepEqual(
        tg.get('tasks').objectAt(i).get('mergedMeta'),
        exp,
        'mergedMeta is merged with task meta'
      );
    });

    tg = job.get('taskGroups').objectAt(1);
    expected = [{ c: 'd' }, {}, {}, {}];

    expected.forEach((exp, i) => {
      assert.deepEqual(
        tg.get('tasks').objectAt(i).get('mergedMeta'),
        exp,
        'mergedMeta is merged with job meta'
      );
    });
  });
});
