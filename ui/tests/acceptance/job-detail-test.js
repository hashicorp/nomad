import { get } from '@ember/object';
import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import moment from 'moment';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

let job;

moduleForAcceptance('Acceptance | job detail', {
  beforeEach() {
    server.create('node');
    job = server.create('job', { type: 'service' });
    visit(`/jobs/${job.id}`);
  },
});

test('visiting /jobs/:job_id', function(assert) {
  assert.equal(currentURL(), `/jobs/${job.id}`);
});

test('breadcrumbs includes job name and link back to the jobs list', function(assert) {
  assert.equal(
    find('[data-test-breadcrumb="Jobs"]').textContent,
    'Jobs',
    'First breadcrumb says jobs'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${job.name}"]`).textContent,
    job.name,
    'Second breadcrumb says the job name'
  );

  click(find('[data-test-breadcrumb="Jobs"]'));
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('the subnav includes links to definition, versions, and deployments when type = service', function(
  assert
) {
  const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
  assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
  assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
  assert.ok(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
});

test('the subnav includes links to definition and versions when type != service', function(assert) {
  job = server.create('job', { type: 'batch' });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    const subnavLabels = findAll('[data-test-tab]').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.notOk(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('the job detail page should contain basic information about the job', function(assert) {
  assert.ok(find('[data-test-job-status]').textContent.includes(job.status), 'Status');
  assert.ok(find('[data-test-job-stat="type"]').textContent.includes(job.type), 'Type');
  assert.ok(find('[data-test-job-stat="priority"]').textContent.includes(job.priority), 'Priority');
  assert.notOk(find('[data-test-job-stat="namespace"]'), 'Namespace is not included');
});

test('the job detail page should list all task groups', function(assert) {
  assert.equal(
    findAll('[data-test-task-group]').length,
    server.db.taskGroups.where({ jobId: job.id }).length
  );
});

test('each row in the task group table should show basic information about the task group', function(
  assert
) {
  const taskGroup = job.taskGroupIds.map(id => server.db.taskGroups.find(id)).sortBy('name')[0];
  const taskGroupRow = find('[data-test-task-group]');
  const tasks = server.db.tasks.where({ taskGroupId: taskGroup.id });
  const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

  assert.equal(
    taskGroupRow.querySelector('[data-test-task-group-name]').textContent.trim(),
    taskGroup.name,
    'Name'
  );
  assert.equal(
    taskGroupRow.querySelector('[data-test-task-group-count]').textContent.trim(),
    taskGroup.count,
    'Count'
  );
  assert.equal(
    taskGroupRow.querySelector('[data-test-task-group-cpu]').textContent.trim(),
    `${sum(tasks, 'Resources.CPU')} MHz`,
    'Reserved CPU'
  );
  assert.equal(
    taskGroupRow.querySelector('[data-test-task-group-mem]').textContent.trim(),
    `${sum(tasks, 'Resources.MemoryMB')} MiB`,
    'Reserved Memory'
  );
  assert.equal(
    taskGroupRow.querySelector('[data-test-task-group-disk]').textContent.trim(),
    `${taskGroup.ephemeralDisk.SizeMB} MiB`,
    'Reserved Disk'
  );
});

test('the allocations diagram lists all allocation status figures', function(assert) {
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
    find('[data-test-legend-value="queued"]').textContent,
    statusCounts.queued,
    `${statusCounts.queued} are queued`
  );

  assert.equal(
    find('[data-test-legend-value="starting"]').textContent,
    statusCounts.starting,
    `${statusCounts.starting} are starting`
  );

  assert.equal(
    find('[data-test-legend-value="running"]').textContent,
    statusCounts.running,
    `${statusCounts.running} are running`
  );

  assert.equal(
    find('[data-test-legend-value="complete"]').textContent,
    statusCounts.complete,
    `${statusCounts.complete} are complete`
  );

  assert.equal(
    find('[data-test-legend-value="failed"]').textContent,
    statusCounts.failed,
    `${statusCounts.failed} are failed`
  );

  assert.equal(
    find('[data-test-legend-value="lost"]').textContent,
    statusCounts.lost,
    `${statusCounts.lost} are lost`
  );
});

test('there is no active deployment section when the job has no active deployment', function(
  assert
) {
  // TODO: it would be better to not visit two different job pages in one test, but this
  // way is much more convenient.
  job = server.create('job', { noActiveDeployment: true, type: 'service' });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.notOk(find('[data-test-active-deployment]'), 'No active deployment');
  });
});

test('the active deployment section shows up for the currently running deployment', function(
  assert
) {
  job = server.create('job', { activeDeployment: true, type: 'service' });
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
    assert.ok(find('[data-test-active-deployment]'), 'Active deployment');
    assert.equal(
      find('[data-test-active-deployment-stat="id"]').textContent.trim(),
      deployment.id.split('-')[0],
      'The active deployment is the most recent running deployment'
    );

    assert.equal(
      find('[data-test-active-deployment-stat="submit-time"]').textContent.trim(),
      moment(version.submitTime / 1000000).fromNow(),
      'Time since the job was submitted is in the active deployment header'
    );

    assert.equal(
      find('[data-test-deployment-metric="canaries"]').textContent.trim(),
      `${sum(taskGroupSummaries, 'placedCanaries')} / ${sum(
        taskGroupSummaries,
        'desiredCanaries'
      )}`,
      'Canaries, both places and desired, are in the metrics'
    );

    assert.equal(
      find('[data-test-deployment-metric="placed"]').textContent.trim(),
      sum(taskGroupSummaries, 'placedAllocs'),
      'Placed allocs aggregates across task groups'
    );

    assert.equal(
      find('[data-test-deployment-metric="desired"]').textContent.trim(),
      sum(taskGroupSummaries, 'desiredTotal'),
      'Desired allocs aggregates across task groups'
    );

    assert.equal(
      find('[data-test-deployment-metric="healthy"]').textContent.trim(),
      sum(taskGroupSummaries, 'healthyAllocs'),
      'Healthy allocs aggregates across task groups'
    );

    assert.equal(
      find('[data-test-deployment-metric="unhealthy"]').textContent.trim(),
      sum(taskGroupSummaries, 'unhealthyAllocs'),
      'Unhealthy allocs aggregates across task groups'
    );

    assert.equal(
      find('[data-test-deployment-notification]').textContent.trim(),
      deployment.statusDescription,
      'Status description is in the metrics block'
    );
  });
});

test('the active deployment section can be expanded to show task groups and allocations', function(
  assert
) {
  job = server.create('job', { activeDeployment: true, type: 'service' });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.notOk(find('[data-test-deployment-task-groups]'), 'Task groups not found');
    assert.notOk(find('[data-test-deployment-allocations]'), 'Allocations not found');
  });

  andThen(() => {
    click('[data-test-deployment-toggle-details]');
  });

  andThen(() => {
    assert.ok(find('[data-test-deployment-task-groups]'), 'Task groups found');
    assert.ok(find('[data-test-deployment-allocations]'), 'Allocations found');
  });
});

test('the evaluations table lists evaluations sorted by modify index', function(assert) {
  job = server.create('job');
  const evaluations = server.db.evaluations
    .where({ jobId: job.id })
    .sortBy('modifyIndex')
    .reverse();

  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.equal(
      findAll('[data-test-evaluation]').length,
      evaluations.length,
      'A row for each evaluation'
    );

    evaluations.forEach((evaluation, index) => {
      const row = findAll('[data-test-evaluation]')[index];
      assert.equal(
        row.querySelector('[data-test-id]').textContent,
        evaluation.id.split('-')[0],
        `Short ID, row ${index}`
      );
    });

    const firstEvaluation = evaluations[0];
    const row = find('[data-test-evaluation]');
    assert.equal(
      row.querySelector('[data-test-priority]').textContent,
      '' + firstEvaluation.priority,
      'Priority'
    );
    assert.equal(
      row.querySelector('[data-test-triggered-by]').textContent,
      firstEvaluation.triggeredBy,
      'Triggered By'
    );
    assert.equal(
      row.querySelector('[data-test-status]').textContent,
      firstEvaluation.status,
      'Status'
    );
  });
});

test('when the job has placement failures, they are called out', function(assert) {
  job = server.create('job', { failedPlacements: true });
  const failedEvaluation = server.db.evaluations
    .where({ jobId: job.id })
    .filter(evaluation => evaluation.failedTGAllocs)
    .sortBy('modifyIndex')
    .reverse()[0];

  const failedTaskGroupNames = Object.keys(failedEvaluation.failedTGAllocs);

  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.ok(find('[data-test-placement-failures]'), 'Placement failures section found');

    const taskGroupLabels = findAll('[data-test-placement-failure-task-group]').map(title =>
      title.textContent.trim()
    );
    failedTaskGroupNames.forEach(name => {
      assert.ok(
        taskGroupLabels.find(label => label.includes(name)),
        `${name} included in placement failures list`
      );
      assert.ok(
        taskGroupLabels.find(label =>
          label.includes(failedEvaluation.failedTGAllocs[name].CoalescedFailures + 1)
        ),
        'The number of unplaced allocs = CoalescedFailures + 1'
      );
    });
  });
});

test('when the job has no placement failures, the placement failures section is gone', function(
  assert
) {
  job = server.create('job', { noFailedPlacements: true });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    assert.notOk(find('[data-test-placement-failures]'), 'Placement failures section not found');
  });
});

test('when the job is not found, an error message is shown, but the URL persists', function(
  assert
) {
  visit('/jobs/not-a-real-job');

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the non-existent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});

moduleForAcceptance('Acceptance | job detail (with namespaces)', {
  beforeEach() {
    server.createList('namespace', 2);
    server.create('node');
    job = server.create('job', { namespaceId: server.db.namespaces[1].name });
    server.createList('job', 3, { namespaceId: server.db.namespaces[0].name });
  },
});

test('when there are namespaces, the job detail page states the namespace for the job', function(
  assert
) {
  const namespace = server.db.namespaces.find(job.namespaceId);
  visit(`/jobs/${job.id}?namespace=${namespace.name}`);

  andThen(() => {
    assert.ok(
      find('[data-test-job-stat="namespace"]').textContent.includes(namespace.name),
      'Namespace included in stats'
    );
  });
});

test('when switching namespaces, the app redirects to /jobs with the new namespace', function(
  assert
) {
  const namespace = server.db.namespaces.find(job.namespaceId);
  const otherNamespace = server.db.namespaces.toArray().find(ns => ns !== namespace).name;
  const label = otherNamespace === 'default' ? 'Default Namespace' : otherNamespace;

  visit(`/jobs/${job.id}?namespace=${namespace.name}`);

  andThen(() => {
    selectChoose('[data-test-namespace-switcher]', label);
  });

  andThen(() => {
    assert.equal(currentURL().split('?')[0], '/jobs', 'Navigated to /jobs');
    const jobs = server.db.jobs
      .where({ namespace: otherNamespace })
      .sortBy('modifyIndex')
      .reverse();
    assert.equal(
      findAll('[data-test-job-row]').length,
      jobs.length,
      'Shows the right number of jobs'
    );
    jobs.forEach((job, index) => {
      const jobRow = findAll('[data-test-job-row]')[index];
      assert.equal(
        jobRow.querySelector('[data-test-job-name]').textContent.trim(),
        job.name,
        `Job ${index} is right`
      );
    });
  });
});
