import Ember from 'ember';
import { click, find, findAll, fillIn, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';

const { $ } = Ember;

let job;
let taskGroup;
let tasks;
let allocations;

const sum = (total, n) => total + n;

moduleForAcceptance('Acceptance | task group detail', {
  beforeEach() {
    server.create('agent');
    server.create('node', 'forceIPv4');

    job = server.create('job', {
      groupsCount: 2,
      createAllocations: false,
    });

    const taskGroups = server.db.taskGroups.where({ jobId: job.id });
    taskGroup = taskGroups[0];

    tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));

    server.create('node', 'forceIPv4');

    allocations = server.createList('allocation', 2, {
      jobId: job.id,
      taskGroup: taskGroup.name,
    });

    // Allocations associated to a different task group on the job to
    // assert that they aren't showing up in on this page in error.
    server.createList('allocation', 3, {
      jobId: job.id,
      taskGroup: taskGroups[1].name,
    });

    // Set a static name to make the search test deterministic
    server.db.allocations.forEach(alloc => {
      alloc.name = 'aaaaa';
    });

    visit(`/jobs/${job.id}/${taskGroup.name}`);
  },
});

test('/jobs/:id/:task-group should list high-level metrics for the allocation', function(assert) {
  const totalCPU = tasks.mapBy('Resources.CPU').reduce(sum, 0);
  const totalMemory = tasks.mapBy('Resources.MemoryMB').reduce(sum, 0);
  const totalDisk = taskGroup.ephemeralDisk.SizeMB;

  assert.equal(
    findAll('.inline-definitions .pair')[0].textContent,
    `# Tasks ${tasks.length}`,
    '# Tasks'
  );
  assert.equal(
    findAll('.inline-definitions .pair')[1].textContent,
    `Reserved CPU ${totalCPU} MHz`,
    'Aggregated CPU reservation for all tasks'
  );
  assert.equal(
    findAll('.inline-definitions .pair')[2].textContent,
    `Reserved Memory ${totalMemory} MiB`,
    'Aggregated Memory reservation for all tasks'
  );
  assert.equal(
    findAll('.inline-definitions .pair')[3].textContent,
    `Reserved Disk ${totalDisk} MiB`,
    'Aggregated Disk reservation for all tasks'
  );
});

test('/jobs/:id/:task-group should have breadcrumbs for job and jobs', function(assert) {
  assert.equal(findAll('.breadcrumb')[0].textContent.trim(), 'Jobs', 'First breadcrumb says jobs');
  assert.equal(
    findAll('.breadcrumb')[1].textContent.trim(),
    job.name,
    'Second breadcrumb says the job name'
  );
  assert.equal(
    findAll('.breadcrumb')[2].textContent.trim(),
    taskGroup.name,
    'Third breadcrumb says the job name'
  );
});

test('/jobs/:id/:task-group first breadcrumb should link to jobs', function(assert) {
  click(findAll('.breadcrumb')[0]);
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('/jobs/:id/:task-group second breadcrumb should link to the job for the task group', function(
  assert
) {
  click(findAll('.breadcrumb')[1]);
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Second breadcrumb links back to the job for the task group'
    );
  });
});

test('/jobs/:id/:task-group should list one page of allocations for the task group', function(
  assert
) {
  const pageSize = 10;

  server.createList('allocation', 10, {
    jobId: job.id,
    taskGroup: taskGroup.name,
  });

  visit('/jobs');
  visit(`/jobs/${job.id}/${taskGroup.name}`);

  andThen(() => {
    assert.ok(
      server.db.allocations.where({ jobId: job.id }).length > pageSize,
      'There are enough allocations to invoke pagination'
    );

    assert.equal(
      findAll('.allocations tbody tr').length,
      pageSize,
      'All allocations for the task group'
    );
  });
});

test('each allocation should show basic information about the allocation', function(assert) {
  const allocation = allocations.sortBy('modifyIndex').reverse()[0];
  const allocationRow = $(findAll('.allocations tbody tr')[0]);

  andThen(() => {
    assert.equal(
      allocationRow
        .find('td:eq(0)')
        .text()
        .trim(),
      allocation.id.split('-')[0],
      'Allocation short id'
    );
    assert.equal(
      allocationRow
        .find('td:eq(1)')
        .text()
        .trim(),
      allocation.modifyIndex,
      'Allocation modify index'
    );
    assert.equal(
      allocationRow
        .find('td:eq(2)')
        .text()
        .trim(),
      allocation.name,
      'Allocation name'
    );
    assert.equal(
      allocationRow
        .find('td:eq(3)')
        .text()
        .trim(),
      allocation.clientStatus,
      'Client status'
    );
    assert.equal(
      allocationRow
        .find('td:eq(4)')
        .text()
        .trim(),
      allocation.jobVersion,
      'Job Version'
    );
    assert.equal(
      allocationRow
        .find('td:eq(5)')
        .text()
        .trim(),
      server.db.nodes.find(allocation.nodeId).id.split('-')[0],
      'Node ID'
    );
  });

  click(allocationRow.find('td:eq(5) a').get(0));

  andThen(() => {
    assert.equal(currentURL(), `/nodes/${allocation.nodeId}`, 'Node links to node page');
  });
});

test('each allocation should show stats about the allocation, retrieved directly from the node', function(
  assert
) {
  const allocation = allocations.sortBy('name')[0];
  const allocationRow = $(findAll('.allocations tbody tr')[0]);
  const allocStats = server.db.clientAllocationStats.find(allocation.id);
  const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));

  const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
  const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

  assert.equal(
    allocationRow
      .find('td:eq(6)')
      .text()
      .trim(),
    Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
    'CPU %'
  );

  assert.equal(
    allocationRow.find('td:eq(6) .tooltip').attr('aria-label'),
    `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
    'Detailed CPU information is in a tooltip'
  );

  assert.equal(
    allocationRow
      .find('td:eq(7)')
      .text()
      .trim(),
    allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
    'Memory used'
  );

  assert.equal(
    allocationRow.find('td:eq(7) .tooltip').attr('aria-label'),
    `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
    'Detailed memory information is in a tooltip'
  );

  const node = server.db.nodes.find(allocation.nodeId);
  const nodeStatsUrl = `//${node.httpAddr}/v1/client/allocation/${allocation.id}/stats`;

  assert.ok(
    server.pretender.handledRequests.some(req => req.url === nodeStatsUrl),
    `Requests ${nodeStatsUrl}`
  );
});

test('when the allocation search has no matches, there is an empty message', function(assert) {
  fillIn('.search-box input', 'zzzzzz');

  andThen(() => {
    assert.ok(find('.allocations .empty-message'));
    assert.equal(find('.allocations .empty-message-headline').textContent, 'No Matches');
  });
});
