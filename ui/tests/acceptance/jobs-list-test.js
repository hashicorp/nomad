import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

moduleForAcceptance('Acceptance | jobs list', {
  beforeEach() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');
  },
});

test('visiting /jobs', function(assert) {
  JobsList.visit();

  andThen(() => {
    assert.equal(currentURL(), '/jobs');
  });
});

test('/jobs should list the first page of jobs sorted by modify index', function(assert) {
  const jobsCount = JobsList.pageSize + 1;
  server.createList('job', jobsCount, { createAllocations: false });

  JobsList.visit();

  andThen(() => {
    const sortedJobs = server.db.jobs.sortBy('modifyIndex').reverse();
    assert.equal(JobsList.jobs.length, JobsList.pageSize);
    JobsList.jobs.forEach((job, index) => {
      assert.equal(job.name, sortedJobs[index].name, 'Jobs are ordered');
    });
  });
});

test('each job row should contain information about the job', function(assert) {
  server.createList('job', 2);
  const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];
  const taskGroups = server.db.taskGroups.where({ jobId: job.id });

  JobsList.visit();

  andThen(() => {
    const jobRow = JobsList.jobs.objectAt(0);

    assert.equal(jobRow.name, job.name, 'Name');
    assert.equal(jobRow.link, `/ui/jobs/${job.id}`, 'Detail Link');
    assert.equal(jobRow.status, job.status, 'Status');
    assert.equal(jobRow.type, typeForJob(job), 'Type');
    assert.equal(jobRow.priority, job.priority, 'Priority');
    andThen(() => {
      assert.equal(jobRow.taskGroups, taskGroups.length, '# Groups');
    });
  });
});

test('each job row should link to the corresponding job', function(assert) {
  server.create('job');
  const job = server.db.jobs[0];

  JobsList.visit();

  andThen(() => {
    JobsList.jobs.objectAt(0).clickName();
  });

  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`);
  });
});

test('the new job button transitions to the new job page', function(assert) {
  JobsList.visit();

  andThen(() => {
    JobsList.runJob();
  });

  andThen(() => {
    assert.equal(currentURL(), '/jobs/run');
  });
});

test('when there are no jobs, there is an empty message', function(assert) {
  JobsList.visit();

  andThen(() => {
    assert.ok(JobsList.isEmpty, 'There is an empty message');
    assert.equal(JobsList.emptyState.headline, 'No Jobs', 'The message is appropriate');
  });
});

test('when there are jobs, but no matches for a search result, there is an empty message', function(assert) {
  server.create('job', { name: 'cat 1' });
  server.create('job', { name: 'cat 2' });

  JobsList.visit();

  andThen(() => {
    JobsList.search('dog');
  });

  andThen(() => {
    assert.ok(JobsList.isEmpty, 'The empty message is shown');
    assert.equal(JobsList.emptyState.headline, 'No Matches', 'The message is appropriate');
  });
});

test('when the namespace query param is set, only matching jobs are shown and the namespace value is forwarded to app state', function(assert) {
  server.createList('namespace', 2);
  const job1 = server.create('job', { namespaceId: server.db.namespaces[0].id });
  const job2 = server.create('job', { namespaceId: server.db.namespaces[1].id });

  JobsList.visit();

  andThen(() => {
    assert.equal(JobsList.jobs.length, 1, 'One job in the default namespace');
    assert.equal(JobsList.jobs.objectAt(0).name, job1.name, 'The correct job is shown');
  });

  const secondNamespace = server.db.namespaces[1];
  JobsList.visit({ namespace: secondNamespace.id });

  andThen(() => {
    assert.equal(JobsList.jobs.length, 1, `One job in the ${secondNamespace.name} namespace`);
    assert.equal(JobsList.jobs.objectAt(0).name, job2.name, 'The correct job is shown');
  });
});

test('when accessing jobs is forbidden, show a message with a link to the tokens page', function(assert) {
  server.pretender.get('/v1/jobs', () => [403, {}, null]);

  JobsList.visit();

  andThen(() => {
    assert.equal(JobsList.error.title, 'Not Authorized');
  });

  andThen(() => {
    JobsList.error.seekHelp();
  });

  andThen(() => {
    assert.equal(currentURL(), '/settings/tokens');
  });
});

function typeForJob(job) {
  return job.periodic ? 'periodic' : job.parameterized ? 'parameterized' : job.type;
}
