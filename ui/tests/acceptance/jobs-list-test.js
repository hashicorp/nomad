import Ember from 'ember';
import { click, find, findAll, currentURL, visit, fillIn } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { $ } = Ember;

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
    assert.equal(findAll('.job-row').length, pageSize);
    for (var jobNumber = 0; jobNumber < pageSize; jobNumber++) {
      assert.equal(
        $(`.job-row:eq(${jobNumber}) td:eq(0)`).text(),
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
    const jobRow = $(findAll('.job-row')[0]);

    assert.equal(jobRow.find('td:eq(0)').text(), job.name, 'Name');
    assert.equal(jobRow.find('td:eq(0) a').attr('href'), `/ui/jobs/${job.id}`, 'Detail Link');
    assert.equal(
      jobRow
        .find('td:eq(1)')
        .text()
        .trim(),
      job.status,
      'Status'
    );
    assert.equal(jobRow.find('td:eq(2)').text(), job.type, 'Type');
    assert.equal(jobRow.find('td:eq(3)').text(), job.priority, 'Priority');
    andThen(() => {
      assert.equal(
        jobRow
          .find('td:eq(4)')
          .text()
          .trim(),
        taskGroups.length,
        '# Groups'
      );
    });
  });
});

test('each job row should link to the corresponding job', function(assert) {
  server.create('job');
  const job = server.db.jobs[0];

  visit('/jobs');

  andThen(() => {
    click($('.job-row:eq(0) td:eq(0) a').get(0));
  });

  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`);
  });
});

test('when there are no jobs, there is an empty message', function(assert) {
  visit('/jobs');

  andThen(() => {
    assert.ok(find('.empty-message'));
    assert.equal(find('.empty-message-headline').textContent, 'No Jobs');
  });
});

test('when there are jobs, but no matches for a search result, there is an empty message', function(
  assert
) {
  server.create('job', { name: 'cat 1' });
  server.create('job', { name: 'cat 2' });

  visit('/jobs');

  andThen(() => {
    fillIn('.search-box input', 'dog');
  });

  andThen(() => {
    assert.ok(find('.empty-message'));
    assert.equal(find('.empty-message-headline').textContent, 'No Matches');
  });
});

test('when the namespace query param is set, only matching jobs are shown and the namespace value is forwarded to app state', function(
  assert
) {
  server.createList('namespace', 2);
  const job1 = server.create('job', { namespaceId: server.db.namespaces[0].id });
  const job2 = server.create('job', { namespaceId: server.db.namespaces[1].id });

  visit('/jobs');

  andThen(() => {
    assert.equal(findAll('.job-row').length, 1, 'One job in the default namespace');
    assert.equal(find('.job-row td').textContent, job1.name, 'The correct job is shown');
  });

  const secondNamespace = server.db.namespaces[1];
  visit(`/jobs?namespace=${secondNamespace.id}`);

  andThen(() => {
    assert.equal(findAll('.job-row').length, 1, `One job in the ${secondNamespace.name} namespace`);
    assert.equal(find('.job-row td').textContent, job2.name, 'The correct job is shown');
  });
});

test('when accessing jobs is forbidden, show a message with a link to the tokens page', function(
  assert
) {
  server.pretender.get('/v1/jobs', () => [403, {}, null]);

  visit('/jobs');

  andThen(() => {
    assert.equal(find('.empty-message-headline').textContent, 'Not Authorized');
  });

  andThen(() => {
    click('.empty-message-body a');
  });

  andThen(() => {
    assert.equal(currentURL(), '/settings/tokens');
  });
});
