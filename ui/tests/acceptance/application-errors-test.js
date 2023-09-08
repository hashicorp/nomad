/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import Job from 'nomad-ui/tests/pages/jobs/detail';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

module('Acceptance | application errors ', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    faker.seed(1);
    server.create('agent');
    server.create('node-pool');
    server.create('node');
    server.create('job');
  });

  test('it passes an accessibility audit', async function (assert) {
    assert.expect(1);

    server.pretender.get('/v1/nodes', () => [500, {}, null]);
    await ClientsList.visit();
    await a11yAudit(assert);
    await percySnapshot(assert);
  });

  test('transitioning away from an error page resets the global error', async function (assert) {
    server.pretender.get('/v1/nodes', () => [500, {}, null]);

    await ClientsList.visit();
    assert.ok(ClientsList.error.isPresent, 'Application has errored');

    await JobsList.visit();
    assert.notOk(
      JobsList.error.isPresent,
      'Application is no longer in an error state'
    );
  });

  test('the 403 error page links to the ACL tokens page', async function (assert) {
    assert.expect(3);
    const job = server.db.jobs[0];

    server.pretender.get(`/v1/job/${job.id}`, () => [403, {}, null]);

    await Job.visit({ id: job.id });

    assert.ok(Job.error.isPresent, 'Error message is shown');
    assert.equal(Job.error.title, 'Not Authorized', 'Error message is for 403');
    await percySnapshot(assert);

    await Job.error.seekHelp();
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Error message contains a link to the tokens page'
    );
  });

  test('the no leader error state gets its own error message', async function (assert) {
    assert.expect(2);
    server.pretender.get('/v1/jobs', () => [500, {}, 'No cluster leader']);

    await JobsList.visit();

    assert.ok(JobsList.error.isPresent, 'An error is shown');
    assert.equal(
      JobsList.error.title,
      'No Cluster Leader',
      'The error is specifically for the lack of a cluster leader'
    );
    await percySnapshot(assert);
  });

  test('error pages include links to the jobs, clients and auth pages', async function (assert) {
    await visit('/a/non-existent/page');

    assert.ok(JobsList.error.isPresent, 'An error is shown');

    await JobsList.error.gotoJobs();
    assert.equal(currentURL(), '/jobs', 'Now on the jobs page');
    assert.notOk(JobsList.error.isPresent, 'The error is gone now');

    await visit('/a/non-existent/page');
    assert.ok(JobsList.error.isPresent, 'An error is shown');

    await JobsList.error.gotoClients();
    assert.equal(currentURL(), '/clients', 'Now on the clients page');
    assert.notOk(JobsList.error.isPresent, 'The error is gone now');

    await visit('/a/non-existent/page');
    assert.ok(JobsList.error.isPresent, 'An error is shown');

    await JobsList.error.gotoSignin();
    assert.equal(currentURL(), '/settings/tokens', 'Now on the sign-in page');
    assert.notOk(JobsList.error.isPresent, 'The error is gone now');
  });
});
