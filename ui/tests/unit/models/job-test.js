import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

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
});
