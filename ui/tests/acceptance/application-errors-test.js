import { currentURL, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import Job from 'nomad-ui/tests/pages/jobs/detail';

module('Acceptance | application errors ', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('agent');
    server.create('node');
    server.create('job');
  });

  test('transitioning away from an error page resets the global error', async function(assert) {
    server.pretender.get('/v1/nodes', () => [500, {}, null]);

    await ClientsList.visit();
    assert.ok(ClientsList.error.isPresent, 'Application has errored');

    await JobsList.visit();
    assert.notOk(JobsList.error.isPresent, 'Application is no longer in an error state');
  });

  test('the 403 error page links to the ACL tokens page', async function(assert) {
    const job = server.db.jobs[0];

    server.pretender.get(`/v1/job/${job.id}`, () => [403, {}, null]);

    await Job.visit({ id: job.id });

    assert.ok(Job.error.isPresent, 'Error message is shown');
    assert.equal(Job.error.title, 'Not Authorized', 'Error message is for 403');

    await Job.error.seekHelp();
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Error message contains a link to the tokens page'
    );
  });

  test('the no leader error state gets its own error message', async function(assert) {
    server.pretender.get('/v1/jobs', () => [500, {}, 'No cluster leader']);

    await JobsList.visit();

    assert.ok(JobsList.error.isPresent, 'An error is shown');
    assert.equal(
      JobsList.error.title,
      'No Cluster Leader',
      'The error is specifically for the lack of a cluster leader'
    );
  });

  test('error pages include links to the jobs and clients pages', async function(assert) {
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
  });
});
