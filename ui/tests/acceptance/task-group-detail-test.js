import { click, find, findAll, fillIn, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import moment from 'moment';

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

    // Mark the first alloc as rescheduled
    allocations[0].update({
      nextAllocation: allocations[1].id,
    });
    allocations[1].update({
      previousAllocation: allocations[0].id,
    });

    visit(`/jobs/${job.id}/${taskGroup.name}`);
  },
});

test('/jobs/:id/:task-group should list high-level metrics for the allocation', function(assert) {
  const totalCPU = tasks.mapBy('Resources.CPU').reduce(sum, 0);
  const totalMemory = tasks.mapBy('Resources.MemoryMB').reduce(sum, 0);
  const totalDisk = taskGroup.ephemeralDisk.SizeMB;

  assert.equal(
    find('[data-test-task-group-tasks]').textContent,
    `# Tasks ${tasks.length}`,
    '# Tasks'
  );
  assert.equal(
    find('[data-test-task-group-cpu]').textContent,
    `Reserved CPU ${totalCPU} MHz`,
    'Aggregated CPU reservation for all tasks'
  );
  assert.equal(
    find('[data-test-task-group-mem]').textContent,
    `Reserved Memory ${totalMemory} MiB`,
    'Aggregated Memory reservation for all tasks'
  );
  assert.equal(
    find('[data-test-task-group-disk]').textContent,
    `Reserved Disk ${totalDisk} MiB`,
    'Aggregated Disk reservation for all tasks'
  );
});

test('/jobs/:id/:task-group should have breadcrumbs for job and jobs', function(assert) {
  assert.equal(
    find('[data-test-breadcrumb="Jobs"]').textContent.trim(),
    'Jobs',
    'First breadcrumb says jobs'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${job.name}"]`).textContent.trim(),
    job.name,
    'Second breadcrumb says the job name'
  );
  assert.equal(
    find(`[data-test-breadcrumb="${taskGroup.name}"]`).textContent.trim(),
    taskGroup.name,
    'Third breadcrumb says the job name'
  );
});

test('/jobs/:id/:task-group first breadcrumb should link to jobs', function(assert) {
  click('[data-test-breadcrumb="Jobs"]');
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('/jobs/:id/:task-group second breadcrumb should link to the job for the task group', function(assert) {
  click(`[data-test-breadcrumb="${job.name}"]`);
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Second breadcrumb links back to the job for the task group'
    );
  });
});

test('/jobs/:id/:task-group should list one page of allocations for the task group', function(assert) {
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
      findAll('[data-test-allocation]').length,
      pageSize,
      'All allocations for the task group'
    );
  });
});

test('each allocation should show basic information about the allocation', function(assert) {
  const allocation = allocations.sortBy('modifyIndex').reverse()[0];
  const allocationRow = find('[data-test-allocation]');

  andThen(() => {
    assert.equal(
      allocationRow.querySelector('[data-test-short-id]').textContent.trim(),
      allocation.id.split('-')[0],
      'Allocation short id'
    );
    assert.equal(
      allocationRow.querySelector('[data-test-modify-time]').textContent.trim(),
      moment(allocation.modifyTime / 1000000).format('MM/DD HH:mm:ss'),
      'Allocation modify time'
    );
    assert.equal(
      allocationRow.querySelector('[data-test-name]').textContent.trim(),
      allocation.name,
      'Allocation name'
    );
    assert.equal(
      allocationRow.querySelector('[data-test-client-status]').textContent.trim(),
      allocation.clientStatus,
      'Client status'
    );
    assert.equal(
      allocationRow.querySelector('[data-test-job-version]').textContent.trim(),
      allocation.jobVersion,
      'Job Version'
    );
    assert.equal(
      allocationRow.querySelector('[data-test-client]').textContent.trim(),
      server.db.nodes.find(allocation.nodeId).id.split('-')[0],
      'Node ID'
    );
  });

  click(allocationRow.querySelector('[data-test-client] a'));

  andThen(() => {
    assert.equal(currentURL(), `/clients/${allocation.nodeId}`, 'Node links to node page');
  });
});

test('each allocation should show stats about the allocation', function(assert) {
  const allocation = allocations.sortBy('name')[0];
  const allocationRow = find('[data-test-allocation]');
  const allocStats = server.db.clientAllocationStats.find(allocation.id);
  const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));

  const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
  const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

  assert.equal(
    allocationRow.querySelector('[data-test-cpu]').textContent.trim(),
    Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
    'CPU %'
  );

  assert.equal(
    allocationRow.querySelector('[data-test-cpu] .tooltip').getAttribute('aria-label'),
    `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
    'Detailed CPU information is in a tooltip'
  );

  assert.equal(
    allocationRow.querySelector('[data-test-mem]').textContent.trim(),
    allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
    'Memory used'
  );

  assert.equal(
    allocationRow.querySelector('[data-test-mem] .tooltip').getAttribute('aria-label'),
    `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
    'Detailed memory information is in a tooltip'
  );
});

test('when the allocation search has no matches, there is an empty message', function(assert) {
  fillIn('.search-box input', 'zzzzzz');

  andThen(() => {
    assert.ok(find('[data-test-empty-allocations-list]'));
    assert.equal(find('[data-test-empty-allocations-list-headline]').textContent, 'No Matches');
  });
});

test('when the allocation has reschedule events, the allocation row is denoted with an icon', function(assert) {
  const rescheduleRow = find(`[data-test-allocation="${allocations[0].id}"]`);
  const normalRow = find(`[data-test-allocation="${allocations[1].id}"]`);

  assert.ok(
    rescheduleRow.querySelector('[data-test-indicators] .icon'),
    'Reschedule row has an icon'
  );
  assert.notOk(normalRow.querySelector('[data-test-indicators] .icon'), 'Normal row has no icon');
});
