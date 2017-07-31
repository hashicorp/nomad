import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let job;
let taskGroup;
let tasks;
let allocations;

const sum = (total, n) => total + n;

moduleForAcceptance('Acceptance | task group detail', {
  beforeEach() {
    job = server.create('job', {
      groupsCount: 2,
    });

    const taskGroups = server.db.taskGroups.where({ jobId: job.id });
    taskGroup = taskGroups[0];

    tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));

    server.create('node');

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
    find('.level-item.cpu h3').text(),
    `${totalCPU} MHz`,
    'Aggregated CPU reservation for all tasks'
  );
  assert.equal(
    find('.level-item.memory h3').text(),
    `${totalMemory} MiB`,
    'Aggregated Memory reservation for all tasks'
  );
  assert.equal(
    find('.level-item.disk h3').text(),
    `${totalDisk} MiB`,
    'Aggregated Disk reservation for all tasks'
  );
  assert.equal(find('.level-item.tasks h3').text(), tasks.length, '# Tasks');
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

test('/jobs/:id/:task-group should list all allocations for the task group', function(assert) {
  assert.equal(
    find('.allocations tbody tr').length,
    allocations.length,
    'All allocations for the task group'
  );
});

test('each allocation should show basic information about the allocation', function(assert) {
  const allocation = allocations[0];
  const allocationRow = find('.allocations tbody tr:eq(0)');

  assert.equal(allocationRow.find('td:eq(0)').text().trim(), allocation.name, 'Allocation name');
  assert.equal(
    allocationRow.find('td:eq(1)').text().trim(),
    server.db.nodes.find(allocation.nodeId).id.split('-')[0],
    'Node name'
  );
});

test('each allocation should show stats about the allocation, retrieved directly from the node', function(
  assert
) {
  const allocation = allocations[0];
  const allocationRow = find('.allocations tbody tr:eq(0)');

  assert.equal(
    allocationRow.find('td:eq(2)').text().trim(),
    server.db.clientAllocationStats.find(allocation.id).resourceUsage.CpuStats.Percent,
    'CPU %'
  );

  const node = server.db.nodes.find(allocation.nodeId);
  const nodeStatsUrl = `//${node.http_addr}/v1/client/allocation/${allocation.id}/stats`;

  assert.ok(
    server.pretender.handledRequests.some(req => req.url === nodeStatsUrl),
    `Requests ${nodeStatsUrl}`
  );
});
