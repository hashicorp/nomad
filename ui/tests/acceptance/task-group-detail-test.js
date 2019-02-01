import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import TaskGroup from 'nomad-ui/tests/pages/jobs/job/task-group';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
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
      clientStatus: 'running',
    });

    // Allocations associated to a different task group on the job to
    // assert that they aren't showing up in on this page in error.
    server.createList('allocation', 3, {
      jobId: job.id,
      taskGroup: taskGroups[1].name,
      clientStatus: 'running',
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

    TaskGroup.visit({ id: job.id, name: taskGroup.name });
  },
});

test('/jobs/:id/:task-group should list high-level metrics for the allocation', function(assert) {
  const totalCPU = tasks.mapBy('Resources.CPU').reduce(sum, 0);
  const totalMemory = tasks.mapBy('Resources.MemoryMB').reduce(sum, 0);
  const totalDisk = taskGroup.ephemeralDisk.SizeMB;

  assert.equal(TaskGroup.tasksCount, `# Tasks ${tasks.length}`, '# Tasks');
  assert.equal(
    TaskGroup.cpu,
    `Reserved CPU ${totalCPU} MHz`,
    'Aggregated CPU reservation for all tasks'
  );
  assert.equal(
    TaskGroup.mem,
    `Reserved Memory ${totalMemory} MiB`,
    'Aggregated Memory reservation for all tasks'
  );
  assert.equal(
    TaskGroup.disk,
    `Reserved Disk ${totalDisk} MiB`,
    'Aggregated Disk reservation for all tasks'
  );
});

test('/jobs/:id/:task-group should have breadcrumbs for job and jobs', function(assert) {
  assert.equal(TaskGroup.breadcrumbFor('jobs.index').text, 'Jobs', 'First breadcrumb says jobs');
  assert.equal(
    TaskGroup.breadcrumbFor('jobs.job.index').text,
    job.name,
    'Second breadcrumb says the job name'
  );
  assert.equal(
    TaskGroup.breadcrumbFor('jobs.job.task-group').text,
    taskGroup.name,
    'Third breadcrumb says the job name'
  );
});

test('/jobs/:id/:task-group first breadcrumb should link to jobs', function(assert) {
  TaskGroup.breadcrumbFor('jobs.index').visit();
  andThen(() => {
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });
});

test('/jobs/:id/:task-group second breadcrumb should link to the job for the task group', function(assert) {
  TaskGroup.breadcrumbFor('jobs.job.index').visit();
  andThen(() => {
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Second breadcrumb links back to the job for the task group'
    );
  });
});

test('/jobs/:id/:task-group should list one page of allocations for the task group', function(assert) {
  server.createList('allocation', TaskGroup.pageSize, {
    jobId: job.id,
    taskGroup: taskGroup.name,
    clientStatus: 'running',
  });

  JobsList.visit();
  TaskGroup.visit({ id: job.id, name: taskGroup.name });

  andThen(() => {
    assert.ok(
      server.db.allocations.where({ jobId: job.id }).length > TaskGroup.pageSize,
      'There are enough allocations to invoke pagination'
    );

    assert.equal(
      TaskGroup.allocations.length,
      TaskGroup.pageSize,
      'All allocations for the task group'
    );
  });
});

test('each allocation should show basic information about the allocation', function(assert) {
  const allocation = allocations.sortBy('modifyIndex').reverse()[0];
  const allocationRow = TaskGroup.allocations.objectAt(0);

  andThen(() => {
    assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'Allocation short id');
    assert.equal(
      allocationRow.createTime,
      moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ'),
      'Allocation create time'
    );
    assert.equal(
      allocationRow.modifyTime,
      moment(allocation.modifyTime / 1000000).fromNow(),
      'Allocation modify time'
    );
    assert.equal(allocationRow.status, allocation.clientStatus, 'Client status');
    assert.equal(allocationRow.jobVersion, allocation.jobVersion, 'Job Version');
    assert.equal(
      allocationRow.client,
      server.db.nodes.find(allocation.nodeId).id.split('-')[0],
      'Node ID'
    );
  });

  allocationRow.visitClient();

  andThen(() => {
    assert.equal(currentURL(), `/clients/${allocation.nodeId}`, 'Node links to node page');
  });
});

test('each allocation should show stats about the allocation', function(assert) {
  const allocation = allocations.sortBy('name')[0];
  const allocationRow = TaskGroup.allocations.objectAt(0);

  const allocStats = server.db.clientAllocationStats.find(allocation.id);
  const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));

  const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
  const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

  assert.equal(
    allocationRow.cpu,
    Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
    'CPU %'
  );

  assert.equal(
    allocationRow.cpuTooltip,
    `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
    'Detailed CPU information is in a tooltip'
  );

  assert.equal(
    allocationRow.mem,
    allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
    'Memory used'
  );

  assert.equal(
    allocationRow.memTooltip,
    `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
    'Detailed memory information is in a tooltip'
  );
});

test('when the allocation search has no matches, there is an empty message', function(assert) {
  TaskGroup.search('zzzzzz');

  andThen(() => {
    assert.ok(TaskGroup.isEmpty, 'Empty state is shown');
    assert.equal(
      TaskGroup.emptyState.headline,
      'No Matches',
      'Empty state has an appropriate message'
    );
  });
});

test('when the allocation has reschedule events, the allocation row is denoted with an icon', function(assert) {
  const rescheduleRow = TaskGroup.allocationFor(allocations[0].id);
  const normalRow = TaskGroup.allocationFor(allocations[1].id);

  assert.ok(rescheduleRow.rescheduled, 'Reschedule row has a reschedule icon');
  assert.notOk(normalRow.rescheduled, 'Normal row has no reschedule icon');
});

test('when the job for the task group is not found, an error message is shown, but the URL persists', function(assert) {
  TaskGroup.visit({ id: 'not-a-real-job', name: 'not-a-real-task-group' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/not-a-real-task-group', 'The URL persists');
    assert.ok(TaskGroup.error.isPresent, 'Error message is shown');
    assert.equal(TaskGroup.error.title, 'Not Found', 'Error message is for 404');
  });
});

test('when the task group is not found on the job, an error message is shown, but the URL persists', function(assert) {
  TaskGroup.visit({ id: job.id, name: 'not-a-real-task-group' });

  andThen(() => {
    assert.ok(
      server.pretender.handledRequests
        .filterBy('status', 200)
        .mapBy('url')
        .includes(`/v1/job/${job.id}`),
      'A request to the job is made and succeeds'
    );
    assert.equal(currentURL(), `/jobs/${job.id}/not-a-real-task-group`, 'The URL persists');
    assert.ok(TaskGroup.error.isPresent, 'Error message is shown');
    assert.equal(TaskGroup.error.title, 'Not Found', 'Error message is for 404');
  });
});
