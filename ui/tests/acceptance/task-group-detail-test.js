import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let job;
let taskGroup;
let tasks;
let allocations;

const sum = (total, n) => total + n;

moduleForAcceptance('Acceptance | task group detail', {
  beforeEach() {
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

    visit(`/jobs/${job.id}/${taskGroup.name}`);
  },
});

test('/jobs/:id/:task-group should list high-level metrics for the allocation', function(assert) {
  const totalCPU = tasks.mapBy('Resources.CPU').reduce(sum, 0);
  const totalMemory = tasks.mapBy('Resources.MemoryMB').reduce(sum, 0);
  const totalDisk = tasks.mapBy('Resources.DiskMB').reduce(sum, 0);

  assert.equal(
    find('.inline-definitions .pair:eq(0)').text(),
    `# Tasks ${tasks.length}`,
    '# Tasks'
  );
  assert.equal(
    find('.inline-definitions .pair:eq(1)').text(),
    `Reserved CPU ${totalCPU} MHz`,
    'Aggregated CPU reservation for all tasks'
  );
  assert.equal(
    find('.inline-definitions .pair:eq(2)').text(),
    `Reserved Memory ${totalMemory} MiB`,
    'Aggregated Memory reservation for all tasks'
  );
  assert.equal(
    find('.inline-definitions .pair:eq(3)').text(),
    `Reserved Disk ${totalDisk} MiB`,
    'Aggregated Disk reservation for all tasks'
  );
});

test('/jobs/:id/:task-group should have breadcrumbs for job and jobs', function(assert) {
  assert.equal(find('.breadcrumb:eq(0)').text(), 'Jobs', 'First breadcrumb says jobs');
  assert.equal(find('.breadcrumb:eq(1)').text(), job.name, 'Second breadcrumb says the job name');
  assert.equal(
    find('.breadcrumb:eq(2)').text(),
    taskGroup.name,
    'Third breadcrumb says the job name'
  );
});

test('/jobs/:id/:task-group first breadcrumb should link to jobs', function(assert) {
  click('.breadcrumb:eq(0)');
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('/jobs/:id/:task-group second breadcrumb should link to the job for the task group', function(
  assert
) {
  click('.breadcrumb:eq(1)');
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
      find('.allocations tbody tr').length,
      pageSize,
      'All allocations for the task group'
    );
  });
});

test('each allocation should show basic information about the allocation', function(assert) {
  const allocation = allocations.sortBy('name')[0];
  const allocationRow = find('.allocations tbody tr:eq(0)');

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
    allocation.name,
    'Allocation name'
  );
  assert.equal(
    allocationRow
      .find('td:eq(2)')
      .text()
      .trim(),
    allocation.clientStatus,
    'Client status'
  );
  assert.equal(
    allocationRow
      .find('td:eq(3)')
      .text()
      .trim(),
    server.db.nodes.find(allocation.nodeId).id.split('-')[0],
    'Node name'
  );
});

test('each allocation should show stats about the allocation, retrieved directly from the node', function(
  assert
) {
  const allocation = allocations.sortBy('name')[0];
  const allocationRow = find('.allocations tbody tr:eq(0)');

  assert.equal(
    allocationRow
      .find('td:eq(4)')
      .text()
      .trim(),
    server.db.clientAllocationStats.find(allocation.id).resourceUsage.CpuStats.Percent,
    'CPU %'
  );

  assert.equal(
    allocationRow
      .find('td:eq(5)')
      .text()
      .trim(),
    server.db.clientAllocationStats.find(allocation.id).resourceUsage.MemoryStats.Cache,
    'Memory used'
  );

  const node = server.db.nodes.find(allocation.nodeId);
  const nodeStatsUrl = `//${node.httpAddr}/v1/client/allocation/${allocation.id}/stats`;

  assert.ok(
    server.pretender.handledRequests.some(req => req.url === nodeStatsUrl),
    `Requests ${nodeStatsUrl}`
  );
});
