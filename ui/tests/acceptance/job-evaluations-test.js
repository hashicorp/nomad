import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Evaluations from 'nomad-ui/tests/pages/jobs/job/evaluations';

let job;
let evaluations;

moduleForAcceptance('Acceptance | job evaluations', {
  beforeEach() {
    job = server.create('job', { noFailedPlacements: true, createAllocations: false });
    evaluations = server.db.evaluations.where({ jobId: job.id });

    Evaluations.visit({ id: job.id });
  },
});

test('lists all evaluations for the job', function(assert) {
  assert.equal(Evaluations.evaluations.length, evaluations.length, 'All evaluations are listed');

  const sortedEvaluations = evaluations.sortBy('modifyIndex').reverse();

  Evaluations.evaluations.forEach((evaluation, index) => {
    const shortId = sortedEvaluations[index].id.split('-')[0];
    assert.equal(evaluation.id, shortId, `Evaluation ${index} is ${shortId}`);
  });
});

test('evaluations table is sortable', function(assert) {
  Evaluations.sortBy('priority');

  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/evaluations?sort=priority`,
      'the URL persists the sort parameter'
    );
    const sortedEvaluations = evaluations.sortBy('priority').reverse();
    Evaluations.evaluations.forEach((evaluation, index) => {
      const shortId = sortedEvaluations[index].id.split('-')[0];
      assert.equal(
        evaluation.id,
        shortId,
        `Evaluation ${index} is ${shortId} with priority ${sortedEvaluations[index].priority}`
      );
    });
  });
});
