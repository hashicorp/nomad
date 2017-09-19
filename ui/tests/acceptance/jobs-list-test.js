import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

moduleForAcceptance('Acceptance | jobs list', {
  beforeEach() {
    // Required for placing allocations (a result of creating jobs)
    server.create('node');
  },
});

test('visiting /jobs', function(assert) {
  visit('/jobs');

  andThen(() => {
    assert.equal(currentURL(), '/jobs');
  });
});

test('/jobs should list the first page of jobs sorted by modify index', function(assert) {
  const jobsCount = 11;
  const pageSize = 10;
  server.createList('job', jobsCount, { createAllocations: false });

  visit('/jobs');

  andThen(() => {
    const sortedJobs = server.db.jobs.sortBy('modifyIndex').reverse();
    assert.equal(find('.job-row').length, pageSize);
    for (var jobNumber = 0; jobNumber < pageSize; jobNumber++) {
      assert.equal(
        find(`.job-row:eq(${jobNumber}) td:eq(0)`).text(),
        sortedJobs[jobNumber].name,
        'Jobs are ordered'
      );
    }
  });
});

test('each job row should contain information about the job', function(assert) {
  server.createList('job', 2);
  const job = server.db.jobs.sortBy('modifyIndex').reverse()[0];
  const taskGroups = server.db.taskGroups.where({ jobId: job.id });

  visit('/jobs');

  andThen(() => {
    const jobRow = find('.job-row:eq(0)');

    assert.equal(jobRow.find('td:eq(0)').text(), job.name, 'Name');
    assert.equal(jobRow.find('td:eq(0) a').attr('href'), `/ui/jobs/${job.id}`, 'Detail Link');
    assert.equal(jobRow.find('td:eq(1)').text().trim(), job.status, 'Status');
    assert.equal(jobRow.find('td:eq(2)').text(), job.type, 'Type');
    assert.equal(jobRow.find('td:eq(3)').text(), job.priority, 'Priority');
    assert.equal(jobRow.find('td:eq(4)').text(), taskGroups.length, '# Groups');
  });
});

test('each job row should link to the corresponding job', function(assert) {
  server.create('job');
  const job = server.db.jobs[0];

  visit('/jobs');
  click('.job-row:eq(0) td:eq(0) a');

  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`);
  });
});
