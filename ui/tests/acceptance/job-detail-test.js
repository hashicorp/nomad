import Ember from 'ember';
import moment from 'moment';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { get } = Ember;
const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

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
  assert.ok(
    find('.title .tag:eq(0)')
      .text()
      .includes(job.status),
    'Status'
  );
  assert.ok(
    find('.job-stats span:eq(0)')
      .text()
      .includes(job.type),
    'Type'
  );
  assert.ok(
    find('.job-stats span:eq(1)')
      .text()
      .includes(job.priority),
    'Priority'
  );
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

test('there is no active deployment section when the job has no active deployment', function(
  assert
) {
  // TODO: it would be better to not visit two different job pages in one test, but this
  // way is much more convenient.
  job = server.create('job', { noActiveDeployment: true });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.ok(find('.active-deployment').length === 0, 'No active deployment');
  });
});

test('the active deployment section shows up for the currently running deployment', function(
  assert
) {
  job = server.create('job', { activeDeployment: true });
  const deployment = server.db.deployments.where({ jobId: job.id })[0];
  const taskGroupSummaries = server.db.deploymentTaskGroupSummaries.where({
    deploymentId: deployment.id,
  });
  const version = server.db.jobVersions.findBy({
    jobId: job.id,
    version: deployment.versionNumber,
  });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.ok(find('.active-deployment').length === 1, 'Active deployment');
    assert.equal(
      find('.active-deployment > .boxed-section-head .badge')
        .text()
        .trim(),
      deployment.id.split('-')[0],
      'The active deployment is the most recent running deployment'
    );

    assert.equal(
      find('.active-deployment > .boxed-section-head .submit-time')
        .text()
        .trim(),
      moment(version.submitTime / 1000000).fromNow(),
      'Time since the job was submitted is in the active deployment header'
    );

    assert.equal(
      find('.deployment-metrics .label:contains("Canaries") + .value')
        .text()
        .trim(),
      `${sum(taskGroupSummaries, 'placedCanaries')} / ${sum(
        taskGroupSummaries,
        'desiredCanaries'
      )}`,
      'Canaries, both places and desired, are in the metrics'
    );

    assert.equal(
      find('.deployment-metrics .label:contains("Placed") + .value')
        .text()
        .trim(),
      sum(taskGroupSummaries, 'placedAllocs'),
      'Placed allocs aggregates across task groups'
    );

    assert.equal(
      find('.deployment-metrics .label:contains("Desired") + .value')
        .text()
        .trim(),
      sum(taskGroupSummaries, 'desiredTotal'),
      'Desired allocs aggregates across task groups'
    );

    assert.equal(
      find('.deployment-metrics .label:contains("Healthy") + .value')
        .text()
        .trim(),
      sum(taskGroupSummaries, 'healthyAllocs'),
      'Healthy allocs aggregates across task groups'
    );

    assert.equal(
      find('.deployment-metrics .label:contains("Unhealthy") + .value')
        .text()
        .trim(),
      sum(taskGroupSummaries, 'unhealthyAllocs'),
      'Unhealthy allocs aggregates across task groups'
    );

    assert.equal(
      find('.deployment-metrics .notification')
        .text()
        .trim(),
      deployment.statusDescription,
      'Status description is in the metrics block'
    );
  });
});

test('the active deployment section can be expanded to show task groups and allocations', function(
  assert
) {
  job = server.create('job', { activeDeployment: true });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.ok(
      find('.active-deployment .boxed-section-head:contains("Task Groups")').length === 0,
      'Task groups not found'
    );
    assert.ok(
      find('.active-deployment .boxed-section-head:contains("Allocations")').length === 0,
      'Allocations not found'
    );
  });

  click('.active-deployment-details-toggle');

  andThen(() => {
    assert.ok(
      find('.active-deployment .boxed-section-head:contains("Task Groups")').length === 1,
      'Task groups found'
    );
    assert.ok(
      find('.active-deployment .boxed-section-head:contains("Allocations")').length === 1,
      'Allocations found'
    );
  });
});
