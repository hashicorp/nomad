/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* eslint-disable qunit/no-conditional-assertions */
import { currentURL, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import {
  formatBytes,
  formatHertz,
  formatScheduledBytes,
  formatScheduledHertz,
} from 'nomad-ui/utils/units';
import TaskGroup from 'nomad-ui/tests/pages/jobs/job/task-group';
import Layout from 'nomad-ui/tests/pages/layout';
import pageSizeSelect from './behaviors/page-size-select';
import moment from 'moment';

let job;
let taskGroup;
let tasks;
let allocations;
let managementToken;

const sum = (total, n) => total + n;

module('Acceptance | task group detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');
    server.create('node-pool');
    server.create('node', 'forceIPv4');

    job = server.create('job', {
      groupsCount: 2,
      createAllocations: false,
    });

    const taskGroups = server.db.taskGroups.where({ jobId: job.id });
    taskGroup = taskGroups[0];

    tasks = taskGroup.taskIds.map((id) => server.db.tasks.find(id));

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
    server.db.allocations.forEach((alloc) => {
      alloc.name = 'aaaaa';
    });

    // Mark the first alloc as rescheduled
    allocations[0].update({
      nextAllocation: allocations[1].id,
    });
    allocations[1].update({
      previousAllocation: allocations[0].id,
    });

    managementToken = server.create('token');

    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });
    await a11yAudit(assert);
  });

  test('/jobs/:id/:task-group should list high-level metrics for the allocation', async function (assert) {
    const totalCPU = tasks.mapBy('resources.CPU').reduce(sum, 0);
    const totalMemory = tasks.mapBy('resources.MemoryMB').reduce(sum, 0);
    const totalMemoryMax = tasks
      .map((t) => t.resources.MemoryMaxMB || t.resources.MemoryMB)
      .reduce(sum, 0);
    const totalDisk = taskGroup.ephemeralDisk.SizeMB;

    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.equal(TaskGroup.tasksCount, `# Tasks ${tasks.length}`, '# Tasks');
    assert.equal(
      TaskGroup.cpu,
      `Reserved CPU ${formatScheduledHertz(totalCPU, 'MHz')}`,
      'Aggregated CPU reservation for all tasks'
    );

    let totalMemoryMaxAddendum = '';

    if (totalMemoryMax > totalMemory) {
      totalMemoryMaxAddendum = ` (${formatScheduledBytes(
        totalMemoryMax,
        'MiB'
      )}Max)`;
    }

    assert.equal(
      TaskGroup.mem,
      `Reserved Memory ${formatScheduledBytes(
        totalMemory,
        'MiB'
      )}${totalMemoryMaxAddendum}`,
      'Aggregated Memory reservation for all tasks'
    );
    assert.equal(
      TaskGroup.disk,
      `Reserved Disk ${formatScheduledBytes(totalDisk, 'MiB')}`,
      'Aggregated Disk reservation for all tasks'
    );

    assert.ok(
      document.title.includes(`Task group ${taskGroup.name} - Job ${job.name}`)
    );
  });

  test('/jobs/:id/:task-group should have breadcrumbs for job and jobs', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.equal(
      Layout.breadcrumbFor('jobs.index').text,
      'Jobs',
      'First breadcrumb says jobs'
    );
    assert.equal(
      Layout.breadcrumbFor('jobs.job.index').text,
      `Job ${job.name}`,
      'Second breadcrumb says the job name'
    );
    assert.equal(
      Layout.breadcrumbFor('jobs.job.task-group').text,
      `Task Group ${taskGroup.name}`,
      'Third breadcrumb says the job name'
    );
  });

  test('/jobs/:id/:task-group first breadcrumb should link to jobs', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    await Layout.breadcrumbFor('jobs.index').visit();
    assert.equal(currentURL(), '/jobs', 'First breadcrumb links back to jobs');
  });

  test('/jobs/:id/:task-group second breadcrumb should link to the job for the task group', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    await Layout.breadcrumbFor('jobs.job.index').visit();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}`,
      'Second breadcrumb links back to the job for the task group'
    );
  });

  test('when the user has a client token that has a namespace with a policy to run and scale a job the autoscaler options should be available', async function (assert) {
    window.localStorage.clear();

    const SCALE_AND_WRITE_NAMESPACE = 'scale-and-write-namespace';
    const READ_ONLY_NAMESPACE = 'read-only-namespace';
    const clientToken = server.create('token');

    server.create('namespace', { id: SCALE_AND_WRITE_NAMESPACE });
    const secondNamespace = server.create('namespace', {
      id: READ_ONLY_NAMESPACE,
    });

    job = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: SCALE_AND_WRITE_NAMESPACE,
    });
    const scalingGroup = server.create('task-group', {
      job,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job.update({ taskGroupIds: [scalingGroup.id] });

    const job2 = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: READ_ONLY_NAMESPACE,
    });
    const scalingGroup2 = server.create('task-group', {
      job: job2,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job2.update({ taskGroupIds: [scalingGroup2.id] });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: SCALE_AND_WRITE_NAMESPACE,
            Capabilities: ['scale-job', 'submit-job', 'read-job', 'list-jobs'],
          },
          {
            Name: READ_ONLY_NAMESPACE,
            Capabilities: ['list-jobs', 'read-job'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await TaskGroup.visit({
      id: `${job.id}@${SCALE_AND_WRITE_NAMESPACE}`,
      name: scalingGroup.name,
    });

    assert.equal(
      decodeURIComponent(currentURL()),
      `/jobs/${job.id}@${SCALE_AND_WRITE_NAMESPACE}/scaling`
    );
    assert.notOk(TaskGroup.countStepper.increment.isDisabled);

    await TaskGroup.visit({
      id: `${job2.id}@${secondNamespace.name}`,
      name: scalingGroup2.name,
    });
    assert.equal(
      decodeURIComponent(currentURL()),
      `/jobs/${job2.id}@${READ_ONLY_NAMESPACE}/scaling`
    );
    assert.ok(TaskGroup.countStepper.increment.isDisabled);
  });

  test('/jobs/:id/:task-group should list one page of allocations for the task group', async function (assert) {
    server.createList('allocation', TaskGroup.pageSize, {
      jobId: job.id,
      taskGroup: taskGroup.name,
      clientStatus: 'running',
    });

    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.ok(
      server.db.allocations.where({ jobId: job.id }).length >
        TaskGroup.pageSize,
      'There are enough allocations to invoke pagination'
    );

    assert.equal(
      TaskGroup.allocations.length,
      TaskGroup.pageSize,
      'All allocations for the task group'
    );
  });

  test('each allocation should show basic information about the allocation', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    const allocation = allocations.sortBy('modifyIndex').reverse()[0];
    const allocationRow = TaskGroup.allocations.objectAt(0);

    assert.equal(
      allocationRow.shortId,
      allocation.id.split('-')[0],
      'Allocation short id'
    );
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
    assert.equal(
      allocationRow.status,
      allocation.clientStatus,
      'Client status'
    );
    assert.equal(
      allocationRow.jobVersion,
      allocation.jobVersion,
      'Job Version'
    );
    assert.equal(
      allocationRow.client,
      server.db.nodes.find(allocation.nodeId).id.split('-')[0],
      'Node ID'
    );
    assert.equal(
      allocationRow.volume,
      Object.keys(taskGroup.volumes).length ? 'Yes' : '',
      'Volumes'
    );

    await allocationRow.visitClient();

    assert.equal(
      currentURL(),
      `/clients/${allocation.nodeId}`,
      'Node links to node page'
    );
  });

  test('each allocation should show stats about the allocation', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    const allocation = allocations.sortBy('name')[0];
    const allocationRow = TaskGroup.allocations.objectAt(0);

    const allocStats = server.db.clientAllocationStats.find(allocation.id);
    const tasks = taskGroup.taskIds.map((id) => server.db.tasks.find(id));

    const cpuUsed = tasks.reduce((sum, task) => sum + task.resources.CPU, 0);
    const memoryUsed = tasks.reduce(
      (sum, task) => sum + task.resources.MemoryMB,
      0
    );

    assert.equal(
      allocationRow.cpu,
      Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
      'CPU %'
    );

    const roundedTicks = Math.floor(
      allocStats.resourceUsage.CpuStats.TotalTicks
    );
    assert.equal(
      allocationRow.cpuTooltip,
      `${formatHertz(roundedTicks, 'MHz')} / ${formatHertz(cpuUsed, 'MHz')}`,
      'Detailed CPU information is in a tooltip'
    );

    assert.equal(
      allocationRow.mem,
      allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
      'Memory used'
    );

    assert.equal(
      allocationRow.memTooltip,
      `${formatBytes(allocStats.resourceUsage.MemoryStats.RSS)} / ${formatBytes(
        memoryUsed,
        'MiB'
      )}`,
      'Detailed memory information is in a tooltip'
    );
  });

  test('when the allocation search has no matches, there is an empty message', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    await TaskGroup.search('zzzzzz');

    assert.ok(TaskGroup.isEmpty, 'Empty state is shown');
    assert.equal(
      TaskGroup.emptyState.headline,
      'No Matches',
      'Empty state has an appropriate message'
    );
  });

  test('when the allocation has reschedule events, the allocation row is denoted with an icon', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    const rescheduleRow = TaskGroup.allocationFor(allocations[0].id);
    const normalRow = TaskGroup.allocationFor(allocations[1].id);

    assert.ok(
      rescheduleRow.rescheduled,
      'Reschedule row has a reschedule icon'
    );
    assert.notOk(normalRow.rescheduled, 'Normal row has no reschedule icon');
  });

  test('/jobs/:id/:task-group should present task lifecycles', async function (assert) {
    job = server.create('job', {
      groupsCount: 2,
      groupTaskCount: 3,
    });

    const taskGroups = server.db.taskGroups.where({ jobId: job.id });
    taskGroup = taskGroups[0];

    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.ok(TaskGroup.lifecycleChart.isPresent);
    assert.equal(
      TaskGroup.lifecycleChart.title,
      'Task Lifecycle Configuration'
    );

    tasks = taskGroup.taskIds.map((id) => server.db.tasks.find(id));
    const taskNames = tasks.mapBy('name');

    // This is thoroughly tested in allocation detail tests, so this mostly checks what’s different

    assert.equal(TaskGroup.lifecycleChart.tasks.length, 3);

    TaskGroup.lifecycleChart.tasks.forEach((Task) => {
      assert.ok(taskNames.includes(Task.name));
      assert.notOk(Task.isActive);
      assert.notOk(Task.isFinished);
    });
  });

  test('when the task group depends on volumes, the volumes table is shown', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.ok(TaskGroup.hasVolumes);
    assert.equal(
      TaskGroup.volumes.length,
      Object.keys(taskGroup.volumes).length
    );
  });

  test('when the task group does not depend on volumes, the volumes table is not shown', async function (assert) {
    job = server.create('job', { noHostVolumes: true, shallow: true });
    taskGroup = server.db.taskGroups.where({ jobId: job.id })[0];

    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.notOk(TaskGroup.hasVolumes);
  });

  test('each row in the volumes table lists information about the volume', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    TaskGroup.volumes[0].as((volumeRow) => {
      const volume = taskGroup.volumes[volumeRow.name];
      assert.equal(volumeRow.name, volume.Name);
      assert.equal(volumeRow.type, volume.Type);
      assert.equal(volumeRow.source, volume.Source);
      assert.equal(
        volumeRow.permissions,
        volume.ReadOnly ? 'Read' : 'Read/Write'
      );
    });
  });

  test('the count stepper sends the appropriate POST request', async function (assert) {
    window.localStorage.nomadTokenSecret = managementToken.secretId;

    job = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
    });
    const scalingGroup = server.create('task-group', {
      job,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job.update({ taskGroupIds: [scalingGroup.id] });

    await TaskGroup.visit({ id: job.id, name: scalingGroup.name });
    await TaskGroup.countStepper.increment.click();
    await settled();

    const scaleRequest = server.pretender.handledRequests.find(
      (req) => req.method === 'POST' && req.url.endsWith('/scale')
    );
    const requestBody = JSON.parse(scaleRequest.requestBody);
    assert.equal(requestBody.Target.Group, scalingGroup.name);
    assert.equal(requestBody.Count, scalingGroup.count + 1);
  });

  test('the count stepper is disabled when a deployment is running', async function (assert) {
    window.localStorage.nomadTokenSecret = managementToken.secretId;

    job = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      activeDeployment: true,
    });
    const scalingGroup = server.create('task-group', {
      job,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job.update({ taskGroupIds: [scalingGroup.id] });

    await TaskGroup.visit({ id: job.id, name: scalingGroup.name });

    assert.ok(TaskGroup.countStepper.input.isDisabled);
    assert.ok(TaskGroup.countStepper.increment.isDisabled);
    assert.ok(TaskGroup.countStepper.decrement.isDisabled);
  });

  test('when the job for the task group is not found, an error message is shown, but the URL persists', async function (assert) {
    await TaskGroup.visit({
      id: 'not-a-real-job',
      name: 'not-a-real-task-group',
    });

    assert.equal(
      server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(
      currentURL(),
      '/jobs/not-a-real-job/not-a-real-task-group',
      'The URL persists'
    );
    assert.ok(TaskGroup.error.isPresent, 'Error message is shown');
    assert.equal(
      TaskGroup.error.title,
      'Not Found',
      'Error message is for 404'
    );
  });

  test('when the task group is not found on the job, an error message is shown, but the URL persists', async function (assert) {
    await TaskGroup.visit({ id: job.id, name: 'not-a-real-task-group' });

    assert.ok(
      server.pretender.handledRequests
        .filterBy('status', 200)
        .mapBy('url')
        .includes(`/v1/job/${job.id}`),
      'A request to the job is made and succeeds'
    );
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/not-a-real-task-group`,
      'The URL persists'
    );
    assert.ok(TaskGroup.error.isPresent, 'Error message is shown');
    assert.equal(
      TaskGroup.error.title,
      'Not Found',
      'Error message is for 404'
    );
  });

  pageSizeSelect({
    resourceName: 'allocation',
    pageObject: TaskGroup,
    pageObjectList: TaskGroup.allocations,
    async setup() {
      server.createList('allocation', TaskGroup.pageSize, {
        jobId: job.id,
        taskGroup: taskGroup.name,
        clientStatus: 'running',
      });

      await TaskGroup.visit({ id: job.id, name: taskGroup.name });
    },
  });

  test('when a task group has no scaling events, there is no recent scaling events section', async function (assert) {
    const taskGroupScale = job.jobScale.taskGroupScales.models.find(
      (m) => m.name === taskGroup.name
    );
    taskGroupScale.update({ events: [] });

    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.notOk(TaskGroup.hasScaleEvents);
  });

  test('the recent scaling events section shows all recent scaling events in reverse chronological order', async function (assert) {
    const taskGroupScale = job.jobScale.taskGroupScales.models.find(
      (m) => m.name === taskGroup.name
    );
    taskGroupScale.update({
      events: [
        server.create('scale-event', { error: true }),
        server.create('scale-event', { error: true }),
        server.create('scale-event', { error: true }),
        server.create('scale-event', { error: true }),
        server.create('scale-event', { count: 3, error: false }),
        server.create('scale-event', { count: 1, error: false }),
      ],
    });
    const scaleEvents = taskGroupScale.events.models.sortBy('time').reverse();
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.ok(TaskGroup.hasScaleEvents);
    assert.notOk(TaskGroup.hasScalingTimeline);

    scaleEvents.forEach((scaleEvent, idx) => {
      const ScaleEvent = TaskGroup.scaleEvents[idx];
      assert.equal(
        ScaleEvent.time,
        moment(scaleEvent.time / 1000000).format('MMM DD HH:mm:ss ZZ')
      );
      assert.equal(ScaleEvent.message, scaleEvent.message);

      if (scaleEvent.count != null) {
        assert.equal(ScaleEvent.count, scaleEvent.count);
      }

      if (scaleEvent.error) {
        assert.ok(ScaleEvent.error);
      }

      if (Object.keys(scaleEvent.meta).length) {
        assert.ok(ScaleEvent.isToggleable);
      } else {
        assert.notOk(ScaleEvent.isToggleable);
      }
    });
  });

  test('when a task group has at least two count scaling events and the count scaling events outnumber the non-count scaling events, a timeline is shown in addition to the accordion', async function (assert) {
    const taskGroupScale = job.jobScale.taskGroupScales.models.find(
      (m) => m.name === taskGroup.name
    );
    taskGroupScale.update({
      events: [
        server.create('scale-event', { error: true }),
        server.create('scale-event', { error: true }),
        server.create('scale-event', { count: 7, error: false }),
        server.create('scale-event', { count: 10, error: false }),
        server.create('scale-event', { count: 2, error: false }),
        server.create('scale-event', { count: 3, error: false }),
        server.create('scale-event', { count: 2, error: false }),
        server.create('scale-event', { count: 9, error: false }),
        server.create('scale-event', { count: 1, error: false }),
      ],
    });
    const scaleEvents = taskGroupScale.events.models.sortBy('time').reverse();
    await TaskGroup.visit({ id: job.id, name: taskGroup.name });

    assert.ok(TaskGroup.hasScaleEvents);
    assert.ok(TaskGroup.hasScalingTimeline);

    assert.equal(
      TaskGroup.scalingAnnotations.length,
      scaleEvents.filter((ev) => ev.count == null).length
    );
  });

  testFacet('Status', {
    facet: TaskGroup.facets.status,
    paramName: 'status',
    expectedOptions: [
      'Pending',
      'Running',
      'Complete',
      'Failed',
      'Lost',
      'Unknown',
    ],
    async beforeEach() {
      ['pending', 'running', 'complete', 'failed', 'lost', 'unknown'].forEach(
        (s) => {
          server.createList('allocation', 5, { clientStatus: s });
        }
      );
      await TaskGroup.visit({ id: job.id, name: taskGroup.name });
    },
    filter: (alloc, selection) =>
      alloc.jobId == job.id &&
      alloc.taskGroup == taskGroup.name &&
      selection.includes(alloc.clientStatus),
  });

  testFacet('Client', {
    facet: TaskGroup.facets.client,
    paramName: 'client',
    expectedOptions(allocs) {
      return Array.from(
        new Set(
          allocs
            .filter(
              (alloc) =>
                alloc.jobId == job.id && alloc.taskGroup == taskGroup.name
            )
            .mapBy('nodeId')
            .map((id) => id.split('-')[0])
        )
      ).sort();
    },
    async beforeEach() {
      const nodes = server.createList('node', 3, 'forceIPv4');
      nodes.forEach((node) =>
        server.createList('allocation', 5, {
          nodeId: node.id,
          jobId: job.id,
          taskGroup: taskGroup.name,
        })
      );
      await TaskGroup.visit({ id: job.id, name: taskGroup.name });
    },
    filter: (alloc, selection) =>
      alloc.jobId == job.id &&
      alloc.taskGroup == taskGroup.name &&
      selection.includes(alloc.nodeId.split('-')[0]),
  });
});

function testFacet(
  label,
  { facet, paramName, beforeEach, filter, expectedOptions }
) {
  test(`facet ${label} | the ${label} facet has the correct options`, async function (assert) {
    await beforeEach();
    await facet.toggle();

    let expectation;
    if (typeof expectedOptions === 'function') {
      expectation = expectedOptions(server.db.allocations);
    } else {
      expectation = expectedOptions;
    }

    assert.deepEqual(
      facet.options.map((option) => option.label.trim()),
      expectation,
      'Options for facet are as expected'
    );
  });

  test(`facet ${label} | the ${label} facet filters the allocations list by ${label}`, async function (assert) {
    let option;

    await beforeEach();

    await facet.toggle();
    option = facet.options.objectAt(0);
    await option.toggle();

    const selection = [option.key];
    const expectedAllocs = server.db.allocations
      .filter((alloc) => filter(alloc, selection))
      .sortBy('modifyIndex')
      .reverse();

    TaskGroup.allocations.forEach((alloc, index) => {
      assert.equal(
        alloc.id,
        expectedAllocs[index].id,
        `Allocation at ${index} is ${expectedAllocs[index].id}`
      );
    });
  });

  test(`facet ${label} | selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
    const selection = [];

    await beforeEach();
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.key);
    await option2.toggle();
    selection.push(option2.key);

    const expectedAllocs = server.db.allocations
      .filter((alloc) => filter(alloc, selection))
      .sortBy('modifyIndex')
      .reverse();

    TaskGroup.allocations.forEach((alloc, index) => {
      assert.equal(
        alloc.id,
        expectedAllocs[index].id,
        `Allocation at ${index} is ${expectedAllocs[index].id}`
      );
    });
  });

  test(`facet ${label} | selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
    const selection = [];

    await beforeEach();
    await facet.toggle();

    const option1 = facet.options.objectAt(0);
    const option2 = facet.options.objectAt(1);
    await option1.toggle();
    selection.push(option1.key);
    await option2.toggle();
    selection.push(option2.key);

    assert.equal(
      currentURL(),
      `/jobs/${job.id}/${taskGroup.name}?${paramName}=${encodeURIComponent(
        JSON.stringify(selection)
      )}`,
      'URL has the correct query param key and value'
    );
  });
}
