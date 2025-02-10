/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import sinon from 'sinon';

module('Unit | Model | job', function (hooks) {
  setupTest(hooks);

  test('should expose aggregate allocations derived from task groups', function (assert) {
    const store = this.owner.lookup('service:store');
    let summary;
    run(() => {
      summary = store.createRecord('job-summary', {
        taskGroupSummaries: [
          {
            name: 'one',
            queuedAllocs: 1,
            startingAllocs: 2,
            runningAllocs: 3,
            completeAllocs: 4,
            failedAllocs: 5,
            lostAllocs: 6,
            unknownAllocs: 7,
          },
          {
            name: 'two',
            queuedAllocs: 2,
            startingAllocs: 4,
            runningAllocs: 6,
            completeAllocs: 8,
            failedAllocs: 10,
            lostAllocs: 12,
            unknownAllocs: 14,
          },
          {
            name: 'three',
            queuedAllocs: 3,
            startingAllocs: 6,
            runningAllocs: 9,
            completeAllocs: 12,
            failedAllocs: 15,
            lostAllocs: 18,
            unknownAllocs: 21,
          },
        ],
      });
    });

    const job = run(() =>
      this.owner.lookup('service:store').createRecord('job', {
        summary,
        name: 'example',
        taskGroups: [
          {
            name: 'one',
            count: 0,
            tasks: [],
          },
          {
            name: 'two',
            count: 0,
            tasks: [],
          },
          {
            name: 'three',
            count: 0,
            tasks: [],
          },
        ],
      })
    );

    assert.equal(
      job.get('totalAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.totalAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'totalAllocs is the sum of all group totalAllocs'
    );

    assert.equal(
      job.get('queuedAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.queuedAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'queuedAllocs is the sum of all group queuedAllocs'
    );

    assert.equal(
      job.get('startingAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.startingAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'startingAllocs is the sum of all group startingAllocs'
    );

    assert.equal(
      job.get('runningAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.runningAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'runningAllocs is the sum of all group runningAllocs'
    );

    assert.equal(
      job.get('completeAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.completeAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'completeAllocs is the sum of all group completeAllocs'
    );

    assert.equal(
      job.get('failedAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.failedAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'failedAllocs is the sum of all group failedAllocs'
    );

    assert.equal(
      job.get('lostAllocs'),
      job
        .get('taskGroups')
        .mapBy('summary.lostAllocs')
        .reduce((sum, allocs) => sum + allocs, 0),
      'lostAllocs is the sum of all group lostAllocs'
    );
  });

  test('actions are aggregated from taskgroups tasks', function (assert) {
    const job = run(() =>
      this.owner.lookup('service:store').createRecord('job', {
        name: 'example',
        taskGroups: [
          {
            name: 'one',
            count: 0,
            tasks: [
              {
                name: '1.1',
                actions: [
                  {
                    name: 'one',
                    command: 'date',
                    args: ['+%s'],
                  },
                  {
                    name: 'two',
                    command: 'sh',
                    args: ['-c "echo hello"'],
                  },
                ],
              },
            ],
          },
          {
            name: 'two',
            count: 0,
            tasks: [
              {
                name: '2.1',
              },
            ],
          },
          {
            name: 'three',
            count: 0,
            tasks: [
              {
                name: '3.1',
                actions: [
                  {
                    name: 'one',
                    command: 'date',
                    args: ['+%s'],
                  },
                ],
              },
              {
                name: '3.2',
                actions: [
                  {
                    name: 'one',
                    command: 'date',
                    args: ['+%s'],
                  },
                ],
              },
            ],
          },
        ],
      })
    );

    assert.equal(
      job.get('actions.length'),
      4,
      'Job draws actions from its task groups tasks'
    );

    // Three actions named one, one named two
    assert.equal(
      job.get('actions').filterBy('name', 'one').length,
      3,
      'Job has three actions named one'
    );
    assert.equal(
      job.get('actions').filterBy('name', 'two').length,
      1,
      'Job has one action named two'
    );

    // Job's actions mapped by task.name return 1.1, 1.1, 3.1, 3.2
    assert.equal(
      job.get('actions').mapBy('task.name').length,
      4,
      'Job action fragments surface their task properties'
    );
    assert.equal(
      job
        .get('actions')
        .mapBy('task.name')
        .filter((name) => name === '1.1').length,
      2,
      'Two of the job actions are from task 1.1'
    );
    assert.equal(
      job
        .get('actions')
        .mapBy('task.name')
        .filter((name) => name === '3.1').length,
      1,
      'One of the job actions is from task 3.1'
    );
    assert.equal(
      job
        .get('actions')
        .mapBy('task.name')
        .filter((name) => name === '3.2').length,
      1,
      'One of the job actions is from task 3.2'
    );
  });

  module('#parse', function () {
    test('it parses JSON', async function (assert) {
      const store = this.owner.lookup('service:store');
      const model = store.createRecord('job');

      model.set('_newDefinition', '{"name": "Tomster"}');

      const setIdByPayloadSpy = sinon.spy(model, 'setIdByPayload');

      const result = await model.parse();

      assert.deepEqual(
        model.get('_newDefinitionJSON'),
        { name: 'Tomster' },
        'Sets _newDefinitionJSON correctly'
      );
      assert.ok(
        setIdByPayloadSpy.calledWith({ name: 'Tomster' }),
        'setIdByPayload is called with the parsed JSON'
      );
      assert.deepEqual(result, '{"name": "Tomster"}', 'Returns the JSON input');
    });

    test('it dispatches a POST request to the /parse endpoint (eagerly assumes HCL specification) if JSON parse method errors', async function (assert) {
      assert.expect(2);

      const store = this.owner.lookup('service:store');
      const model = store.createRecord('job');

      model.set('_newDefinition', 'invalidJSON');

      const adapter = store.adapterFor('job');
      adapter.parse = sinon.stub().resolves('invalidJSON');

      await model.parse();

      assert.ok(
        adapter.parse.calledWith('invalidJSON', undefined),
        'adapter parse method should be called'
      );

      assert.deepEqual(
        model.get('_newDefinitionJSON'),
        'invalidJSON',
        '_newDefinitionJSON is set'
      );
    });
  });
});
