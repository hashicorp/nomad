import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

moduleForAcceptance('Acceptance | jobs list');

test('visiting /jobs', function(assert) {
  visit('/jobs');

  andThen(() => {
    assert.equal(currentURL(), '/jobs');
  });
});

test('/jobs should list all jobs', function(assert) {
  const jobsCount = 5;
  server.createList('job', jobsCount);

  visit('/jobs');

  andThen(() => {
    assert.equal(find('.job-row').length, jobsCount);
  });
});

test('each job row should contain information about the job', function(assert) {
  server.createList('job', 2);
  const job = server.db.jobs[0];
  const summary = server.db.jobSummaries.findBy({ jobId: job.id });
  const taskGroups = server.db.taskGroups.where({ jobId: job.id });

  visit('/jobs');

  andThen(() => {
    const jobRow = find('.job-row:eq(0)');

    assert.equal(jobRow.find('td:eq(0)').text(), job.name, 'Name');
    assert.equal(jobRow.find('td:eq(0) a').attr('href'), `/ui/jobs/${job.id}`, 'Detail Link');
    assert.equal(jobRow.find('td:eq(1)').text(), job.type, 'Type');
    assert.equal(jobRow.find('td:eq(2)').text(), job.priority, 'Priority');
    assert.equal(jobRow.find('td:eq(3)').text(), job.status, 'Status');
    assert.equal(jobRow.find('td:eq(5)').text(), taskGroups.length, '# Groups');
    assert.equal(
      jobRow.find('td:eq(7)').text(),
      Object.keys(summary.Summary).reduce(
        (count, groupKey) => summary.Summary[groupKey].Lost + count,
        0
      ),
      '# Lost'
    );
  });
});

test('the high-level metrics include counts based on job status', function(assert) {
  server.createList('job', 15);
  const jobs = server.db.jobs;

  visit('/jobs');

  andThen(() => {
    assert.equal(find('.level-item:eq(0) .title').text(), jobs.length, 'Total');
    assert.equal(
      find('.level-item:eq(1) .title').text(),
      jobs.where({ status: 'pending' }).length,
      'Pending'
    );
    assert.equal(
      find('.level-item:eq(2) .title').text(),
      jobs.where({ status: 'running' }).length,
      'Running'
    );
    assert.equal(
      find('.level-item:eq(3) .title').text(),
      jobs.where({ status: 'dead' }).length,
      'Dead'
    );
  });
});
