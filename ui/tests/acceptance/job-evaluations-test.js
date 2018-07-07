import { test } from 'qunit';
import { findAll, click } from 'ember-native-dom-helpers';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let job;
let evaluations;

moduleForAcceptance('Acceptance | job evaluations', {
  beforeEach() {
    job = server.create('job', { noFailedPlacements: true, createAllocations: false });
    evaluations = server.db.evaluations.where({ jobId: job.id });

    visit(`/jobs/${job.id}/evaluations`);
  },
});

test('lists all evaluations for the job', function(assert) {
  const evaluationRows = findAll('[data-test-evaluation]');
  assert.equal(evaluationRows.length, evaluations.length, 'All evaluations are listed');

  evaluations
    .sortBy('modifyIndex')
    .reverse()
    .forEach((evaluation, index) => {
      const shortId = evaluation.id.split('-')[0];
      assert.equal(
        evaluationRows[index].querySelector('[data-test-id]').textContent.trim(),
        shortId,
        `Evaluation ${index} is ${shortId}`
      );
    });
});

test('evaluations table is sortable', function(assert) {
  click('[data-test-sort-by="priority"]');

  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/evaluations?sort=priority`,
      'the URL persists the sort parameter'
    );
    const evaluationRows = findAll('[data-test-evaluation]');
    evaluations
      .sortBy('priority')
      .reverse()
      .forEach((evaluation, index) => {
        const shortId = evaluation.id.split('-')[0];
        assert.equal(
          evaluationRows[index].querySelector('[data-test-id]').textContent.trim(),
          shortId,
          `Evaluation ${index} is ${shortId} with priority ${evaluation.priority}`
        );
      });
  });
});
