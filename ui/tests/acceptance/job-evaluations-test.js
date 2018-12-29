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

test('when the job for the evaluations is not found, an error message is shown, but the URL persists', function(assert) {
  Evaluations.visit({ id: 'not-a-real-job' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/evaluations', 'The URL persists');
    assert.ok(Evaluations.error.isPresent, 'Error message is shown');
    assert.equal(Evaluations.error.title, 'Not Found', 'Error message is for 404');
  });
});
