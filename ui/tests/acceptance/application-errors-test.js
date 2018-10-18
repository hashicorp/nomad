import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { test } from 'qunit';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import Job from 'nomad-ui/tests/pages/jobs/detail';

moduleForAcceptance('Acceptance | application errors ', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.create('job');
  },
});

test('transitioning away from an error page resets the global error', function(assert) {
  server.pretender.get('/v1/nodes', () => [500, {}, null]);

  ClientsList.visit();

  andThen(() => {
    assert.ok(ClientsList.error.isPresent, 'Application has errored');
  });

  JobsList.visit();

  andThen(() => {
    assert.notOk(JobsList.error.isPresent, 'Application is no longer in an error state');
  });
});

test('the 403 error page links to the ACL tokens page', function(assert) {
  const job = server.db.jobs[0];

  server.pretender.get(`/v1/job/${job.id}`, () => [403, {}, null]);

  Job.visit({ id: job.id });

  andThen(() => {
    assert.ok(Job.error.isPresent, 'Error message is shown');
    assert.equal(Job.error.title, 'Not Authorized', 'Error message is for 403');
  });

  andThen(() => {
    Job.error.seekHelp();
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      '/settings/tokens',
      'Error message contains a link to the tokens page'
    );
  });
});

test('the no leader error state gets its own error message', function(assert) {
  server.pretender.get('/v1/jobs', () => [500, {}, 'No cluster leader']);

  JobsList.visit();

  andThen(() => {
    assert.ok(JobsList.error.isPresent, 'An error is shown');
    assert.equal(
      JobsList.error.title,
      'No Cluster Leader',
      'The error is specifically for the lack of a cluster leader'
    );
  });
});
