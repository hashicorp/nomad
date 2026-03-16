/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
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
    this.server.create('node-pool');
    job = this.server.create('job', {
      noFailedPlacements: true,
      createAllocations: false,
    });
    evaluations = this.server.db.evaluations.where({ jobId: job.id });

    await Evaluations.visit({ id: job.id });
  });

  test('it passes an accessibility audit', async function (assert) {
    await a11yAudit(assert);
  });

  test('lists all evaluations for the job', async function (assert) {
    assert.deepEqual(
      Evaluations.evaluations.length,
      evaluations.length,
      'All evaluations are listed',
    );

    const sortedEvaluations = evaluations.sortBy('modifyIndex').reverse();

    Evaluations.evaluations.forEach((evaluation, index) => {
      const shortId = sortedEvaluations[index].id.split('-')[0];
      assert.deepEqual(
        evaluation.id,
        shortId,
        `Evaluation ${index} is ${shortId}`,
      );
    });

    assert.deepEqual(getPageTitle(), `Job ${job.name} evaluations - Nomad`);
  });

  test('evaluations table is sortable', async function (assert) {
    await Evaluations.sortBy('priority');

    assert.deepEqual(
      currentURL(),
      `/jobs/${job.id}/evaluations?sort=priority`,
      'the URL persists the sort parameter',
    );
    const sortedEvaluations = evaluations.sortBy('priority').reverse();
    Evaluations.evaluations.forEach((evaluation, index) => {
      const shortId = sortedEvaluations[index].id.split('-')[0];
      assert.deepEqual(
        evaluation.id,
        shortId,
        `Evaluation ${index} is ${shortId} with priority ${sortedEvaluations[index].priority}`,
      );
    });
  });

  test('when the job for the evaluations is not found, an error message is shown, but the URL persists', async function (assert) {
    await Evaluations.visit({ id: 'not-a-real-job' });

    assert.deepEqual(
      this.server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made',
    );
    assert.deepEqual(
      currentURL(),
      '/jobs/not-a-real-job/evaluations',
      'The URL persists',
    );
    assert.ok(Evaluations.error.isPresent, 'Error message is shown');
    assert.deepEqual(
      Evaluations.error.title,
      'Not Found',
      'Error message is for 404',
    );
  });
});
