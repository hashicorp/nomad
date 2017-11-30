import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import Ember from 'ember';
import moment from 'moment';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { get, $ } = Ember;
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
  assert.equal(findAll('.breadcrumb')[0].textContent, 'Jobs', 'First breadcrumb says jobs');
  assert.equal(
    findAll('.breadcrumb')[1].textContent,
    job.name,
    'Second breadcrumb says the job name'
  );

  click(findAll('.breadcrumb')[0]);
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('the subnav includes links to definition, versions, and deployments when type = service', function(
  assert
) {
  const subnavLabels = findAll('.tabs.is-subnav a').map(anchor => anchor.textContent);
  assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
  assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
  assert.ok(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
});

test('the subnav includes links to definition and versions when type != service', function(assert) {
  job = server.create('job', { type: 'batch' });
  visit(`/jobs/${job.id}`);

  andThen(() => {
    const subnavLabels = findAll('.tabs.is-subnav a').map(anchor => anchor.textContent);
    assert.ok(subnavLabels.some(label => label === 'Definition'), 'Definition link');
    assert.ok(subnavLabels.some(label => label === 'Versions'), 'Versions link');
    assert.notOk(subnavLabels.some(label => label === 'Deployments'), 'Deployments link');
  });
});

test('the job detail page should contain basic information about the job', function(assert) {
  assert.ok(findAll('.title .tag')[0].textContent.includes(job.status), 'Status');
  assert.ok(findAll('.job-stats span')[0].textContent.includes(job.type), 'Type');
  assert.ok(findAll('.job-stats span')[1].textContent.includes(job.priority), 'Priority');
  assert.notOk(findAll('.job-stats span')[2], 'Namespace is not included');
});

test('the job detail page should list all task groups', function(assert) {
  assert.equal(
    findAll('.task-group-row').length,
    server.db.taskGroups.where({ jobId: job.id }).length
  );
});

test('each row in the task group table should show basic information about the task group', function(
  assert
) {
  const taskGroup = job.taskGroupIds.map(id => server.db.taskGroups.find(id)).sortBy('name')[0];
  const taskGroupRow = $(findAll('.task-group-row')[0]);
  const tasks = server.db.tasks.where({ taskGroupId: taskGroup.id });
  const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

  assert.equal(
    taskGroupRow
      .find('td:eq(0)')
      .text()
      .trim(),
    taskGroup.name,
    'Name'
  );
  assert.equal(
    taskGroupRow
      .find('td:eq(1)')
      .text()
      .trim(),
    taskGroup.count,
    'Count'
  );
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
    `${taskGroup.ephemeralDisk.SizeMB} MiB`,
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
    legend.querySelector('li.queued .value').textContent,
    statusCounts.queued,
    `${statusCounts.queued} are queued`
  );

  assert.equal(
    legend.querySelector('li.starting .value').textContent,
    statusCounts.starting,
    `${statusCounts.starting} are starting`
  );

  assert.equal(
    legend.querySelector('li.running .value').textContent,
    statusCounts.running,
    `${statusCounts.running} are running`
  );

  assert.equal(
    legend.querySelector('li.complete .value').textContent,
    statusCounts.complete,
    `${statusCounts.complete} are complete`
  );

  assert.equal(
    legend.querySelector('li.failed .value').textContent,
    statusCounts.failed,
    `${statusCounts.failed} are failed`
  );

  assert.equal(
    legend.querySelector('li.lost .value').textContent,
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
    assert.ok(findAll('.active-deployment').length === 0, 'No active deployment');
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
    assert.ok(findAll('.active-deployment').length === 1, 'Active deployment');
    assert.equal(
      $('.active-deployment > .boxed-section-head .badge')
        .get(0)
        .textContent.trim(),
      deployment.id.split('-')[0],
      'The active deployment is the most recent running deployment'
    );

    assert.equal(
      $('.active-deployment > .boxed-section-head .submit-time')
        .get(0)
        .textContent.trim(),
      moment(version.submitTime / 1000000).fromNow(),
      'Time since the job was submitted is in the active deployment header'
    );

    assert.equal(
      $('.deployment-metrics .label:contains("Canaries") + .value')
        .get(0)
        .textContent.trim(),
      `${sum(taskGroupSummaries, 'placedCanaries')} / ${sum(
        taskGroupSummaries,
        'desiredCanaries'
      )}`,
      'Canaries, both places and desired, are in the metrics'
    );

    assert.equal(
      $('.deployment-metrics .label:contains("Placed") + .value')
        .get(0)
        .textContent.trim(),
      sum(taskGroupSummaries, 'placedAllocs'),
      'Placed allocs aggregates across task groups'
    );

    assert.equal(
      $('.deployment-metrics .label:contains("Desired") + .value')
        .get(0)
        .textContent.trim(),
      sum(taskGroupSummaries, 'desiredTotal'),
      'Desired allocs aggregates across task groups'
    );

    assert.equal(
      $('.deployment-metrics .label:contains("Healthy") + .value')
        .get(0)
        .textContent.trim(),
      sum(taskGroupSummaries, 'healthyAllocs'),
      'Healthy allocs aggregates across task groups'
    );

    assert.equal(
      $('.deployment-metrics .label:contains("Unhealthy") + .value')
        .get(0)
        .textContent.trim(),
      sum(taskGroupSummaries, 'unhealthyAllocs'),
      'Unhealthy allocs aggregates across task groups'
    );

    assert.equal(
      $('.deployment-metrics .notification')
        .get(0)
        .textContent.trim(),
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
    assert.ok(
      $('.active-deployment .boxed-section-head:contains("Task Groups")').length === 0,
      'Task groups not found'
    );
    assert.ok(
      $('.active-deployment .boxed-section-head:contains("Allocations")').length === 0,
      'Allocations not found'
    );
  });

  andThen(() => {
    click('.active-deployment-details-toggle');
  });

  andThen(() => {
    assert.ok(
      $('.active-deployment .boxed-section-head:contains("Task Groups")').length === 1,
      'Task groups found'
    );
    assert.ok(
      $('.active-deployment .boxed-section-head:contains("Allocations")').length === 1,
      'Allocations found'
    );
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
    assert.ok(find('.error-message'), 'Error message is shown');
    assert.equal(
      find('.error-message .title').textContent,
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
      findAll('.job-stats span')[2].textContent.includes(namespace.name),
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
    selectChoose('.namespace-switcher', label);
  });

  andThen(() => {
    assert.equal(currentURL().split('?')[0], '/jobs', 'Navigated to /jobs');
    const jobs = server.db.jobs
      .where({ namespace: otherNamespace })
      .sortBy('modifyIndex')
      .reverse();
    assert.equal(findAll('.job-row').length, jobs.length, 'Shows the right number of jobs');
    jobs.forEach((job, index) => {
      assert.equal(
        $(findAll('.job-row')[index])
          .find('td:eq(0)')
          .text()
          .trim(),
        job.name,
        `Job ${index} is right`
      );
    });
  });
});
