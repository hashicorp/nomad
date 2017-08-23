import Ember from 'ember';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { get } = Ember;

let job;

moduleForAcceptance('Acceptance | job detail', {
  beforeEach() {
    server.create('node');
    server.create('job');
    job = server.db.jobs[0];
    visit(`/jobs/${job.id}`);
  },
});

test('visiting /jobs/:job_id', function(assert) {
  assert.equal(currentURL(), `/jobs/${job.id}`);
});

test('breadcrumbs includes job name and link back to the jobs list', function(assert) {
  assert.equal(find('.breadcrumb:eq(0)').text(), 'Jobs', 'First breadcrumb says jobs');
  assert.equal(find('.breadcrumb:eq(1)').text(), job.name, 'Second breadcrumb says the job name');

  click('.breadcrumb:eq(0)');
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('the job detail page should contain basic information about the job', function(assert) {
  assert.ok(find('.title .tag:eq(0)').text().includes(job.status), 'Status');
  assert.ok(find('.job-stats span:eq(0)').text().includes(job.type), 'Type');
  assert.ok(find('.job-stats span:eq(1)').text().includes(job.priority), 'Priority');
});

test('the job detail page should list all task groups', function(assert) {
  assert.equal(
    find('.task-group-row').length,
    server.db.taskGroups.where({ jobId: job.id }).length
  );
});

test('each row in the task group table should show basic information about the task group', function(
  assert
) {
  const taskGroup = job.taskGroupIds.map(id => server.db.taskGroups.find(id)).sortBy('name')[0];
  const taskGroupRow = find('.task-group-row:eq(0)');
  const tasks = server.db.tasks.where({ taskGroupId: taskGroup.id });
  const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

  assert.equal(taskGroupRow.find('td:eq(0)').text(), taskGroup.name, 'Name');
  assert.equal(taskGroupRow.find('td:eq(1)').text(), taskGroup.count, 'Count');
  assert.equal(
    taskGroupRow.find('td:eq(3)').text(),
    `${sum(tasks, 'Resources.CPU')} MHz`,
    'Reserved CPU'
  );
  assert.equal(
    taskGroupRow.find('td:eq(4)').text(),
    `${sum(tasks, 'Resources.MemoryMB')} MiB`,
    'Reserved Memory'
  );
  assert.equal(
    taskGroupRow.find('td:eq(5)').text(),
    `${sum(tasks, 'Resources.DiskMB')} MiB`,
    'Reserved Disk'
  );
});

test('the allocations diagram lists all allocation status figures', function(assert) {
  const legend = find('.distribution-bar .legend');
  const jobSummary = server.db.jobSummaries.findBy({ jobId: job.id });
  const statusCounts = Object.keys(jobSummary.Summary).reduce(
    (counts, key) => {
      const group = jobSummary.Summary[key];
      counts.queued += group.Queued;
      counts.starting += group.Starting;
      counts.running += group.Running;
      counts.complete += group.Complete;
      counts.failed += group.Failed;
      counts.lost += group.Lost;
      return counts;
    },
    { queued: 0, starting: 0, running: 0, complete: 0, failed: 0, lost: 0 }
  );

  assert.equal(
    legend.find('li.queued .value').text(),
    statusCounts.queued,
    `${statusCounts.queued} are queued`
  );

  assert.equal(
    legend.find('li.starting .value').text(),
    statusCounts.starting,
    `${statusCounts.starting} are starting`
  );

  assert.equal(
    legend.find('li.running .value').text(),
    statusCounts.running,
    `${statusCounts.running} are running`
  );

  assert.equal(
    legend.find('li.complete .value').text(),
    statusCounts.complete,
    `${statusCounts.complete} are complete`
  );

  assert.equal(
    legend.find('li.failed .value').text(),
    statusCounts.failed,
    `${statusCounts.failed} are failed`
  );

  assert.equal(
    legend.find('li.lost .value').text(),
    statusCounts.lost,
    `${statusCounts.lost} are lost`
  );
});
