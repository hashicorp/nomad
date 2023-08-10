/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Evaluations from 'nomad-ui/tests/pages/jobs/job/evaluations';

let job;
let evaluations;

module('Acceptance | job evaluations', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('node-pool');
    job = server.create('job', {
      noFailedPlacements: true,
      createAllocations: false,
    });
    evaluations = server.db.evaluations.where({ jobId: job.id });

    await Evaluations.visit({ id: job.id });
  });

  test('it passes an accessibility audit', async function (assert) {
    await a11yAudit(assert);
  });

  test('lists all evaluations for the job', async function (assert) {
    assert.equal(
      Evaluations.evaluations.length,
      evaluations.length,
      'All evaluations are listed'
    );

    const sortedEvaluations = evaluations.sortBy('modifyIndex').reverse();

    Evaluations.evaluations.forEach((evaluation, index) => {
      const shortId = sortedEvaluations[index].id.split('-')[0];
      assert.equal(evaluation.id, shortId, `Evaluation ${index} is ${shortId}`);
    });

    assert.equal(document.title, `Job ${job.name} evaluations - Nomad`);
  });

  test('evaluations table is sortable', async function (assert) {
    await Evaluations.sortBy('priority');

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

  test('when the job for the evaluations is not found, an error message is shown, but the URL persists', async function (assert) {
    await Evaluations.visit({ id: 'not-a-real-job' });

    assert.equal(
      server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(
      currentURL(),
      '/jobs/not-a-real-job/evaluations',
      'The URL persists'
    );
    assert.ok(Evaluations.error.isPresent, 'Error message is shown');
    assert.equal(
      Evaluations.error.title,
      'Not Found',
      'Error message is for 404'
    );
  });
});
