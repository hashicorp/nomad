/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
/* Mirage fixtures are random so we can't expect a set number of assertions */
import AdapterError from '@ember-data/adapter/error';
import { run } from '@ember/runloop';
import { currentURL, click, triggerEvent, waitFor } from '@ember/test-helpers';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';
import moment from 'moment';
import formatHost from 'nomad-ui/utils/format-host';
import faker from 'nomad-ui/mirage/faker';

let job;
let node;
let allocation;

module('Acceptance | allocation detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');

    server.create('node-pool');
    node = server.create('node');
    job = server.create('job', {
      groupsCount: 1,
      withGroupServices: true,
      createAllocations: false,
    });
    allocation = server.create('allocation', 'withTaskWithPorts', {
      clientStatus: 'running',
    });

    // Make sure the node has an unhealthy driver
    node.update({
      driver: assign(node.drivers, {
        docker: {
          detected: true,
          healthy: false,
        },
      }),
    });

    // Make sure a task for the allocation depends on the unhealthy driver
    server.schema.tasks.first().update({
      driver: 'docker',
    });

    await Allocation.visit({ id: allocation.id });
  });

  test('it passes an accessibility audit', async function (assert) {
    await a11yAudit(assert);
  });

  test('/allocation/:id should name the allocation and link to the corresponding job and node', async function (assert) {
    assert.ok(
      Allocation.title.includes(allocation.name),
      'Allocation name is in the heading'
    );
    assert.equal(
      Allocation.details.job,
      job.name,
      'Job name is in the subheading'
    );
    assert.equal(
      Allocation.details.client,
      node.id.split('-')[0],
      'Node short id is in the subheading'
    );
    assert.ok(Allocation.execButton.isPresent);

    assert.ok(document.title.includes(`Allocation ${allocation.name} `));

    await Allocation.details.visitJob();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}@default`,
      'Job link navigates to the job'
    );

    await Allocation.visit({ id: allocation.id });

    await Allocation.details.visitClient();
    assert.equal(
      currentURL(),
      `/clients/${node.id}`,
      'Client link navigates to the client'
    );
  });

  test('/allocation/:id should include resource utilization graphs', async function (assert) {
    assert.equal(
      Allocation.resourceCharts.length,
      2,
      'Two resource utilization graphs'
    );
    assert.equal(
      Allocation.resourceCharts.objectAt(0).name,
      'CPU',
      'First chart is CPU'
    );
    assert.equal(
      Allocation.resourceCharts.objectAt(1).name,
      'Memory',
      'Second chart is Memory'
    );
  });

  test('/allocation/:id should present task lifecycles', async function (assert) {
    const job = server.create('job', {
      groupsCount: 1,
      groupTaskCount: 6,
      withGroupServices: true,
      createAllocations: false,
    });

    const allocation = server.create('allocation', 'withTaskWithPorts', {
      clientStatus: 'running',
      jobId: job.id,
    });

    await Allocation.visit({ id: allocation.id });

    assert.ok(Allocation.lifecycleChart.isPresent);
    assert.equal(Allocation.lifecycleChart.title, 'Task Lifecycle Status');
    assert.equal(Allocation.lifecycleChart.phases.length, 4);
    assert.equal(Allocation.lifecycleChart.tasks.length, 6);

    await Allocation.lifecycleChart.tasks[0].visit();

    const prestartEphemeralTask = server.db.taskStates
      .where({ allocationId: allocation.id })
      .sortBy('name')
      .find((taskState) => {
        const task = server.db.tasks.findBy({ name: taskState.name });
        return (
          task.Lifecycle &&
          task.Lifecycle.Hook === 'prestart' &&
          !task.Lifecycle.Sidecar
        );
      });

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${prestartEphemeralTask.name}`
    );
  });

  test('/allocation/:id should list all tasks for the allocation', async function (assert) {
    assert.equal(
      Allocation.tasks.length,
      server.db.taskStates.where({ allocationId: allocation.id }).length,
      'Table lists all tasks'
    );
    assert.notOk(Allocation.isEmpty, 'Task table empty state is not shown');
  });

  test('each task row should list high-level information for the task', async function (assert) {
    const job = server.create('job', {
      groupsCount: 1,
      groupTaskCount: 3,
      withGroupServices: true,
      createAllocations: false,
    });

    const allocation = server.create('allocation', 'withTaskWithPorts', {
      clientStatus: 'running',
      jobId: job.id,
    });

    const taskGroup = server.schema.taskGroups.where({
      jobId: allocation.jobId,
      name: allocation.taskGroup,
    }).models[0];

    // Set the expected task states.
    const states = ['running', 'pending', 'dead'];
    server.db.taskStates
      .where({ allocationId: allocation.id })
      .sortBy('name')
      .forEach((task, i) => {
        server.db.taskStates.update(task.id, { state: states[i] });
      });

    await Allocation.visit({ id: allocation.id });

    Allocation.tasks.forEach((taskRow, i) => {
      const task = server.db.taskStates
        .where({ allocationId: allocation.id })
        .sortBy('name')[i];
      const events = server.db.taskEvents.where({ taskStateId: task.id });
      const event = events[events.length - 1];

      const jobTask = taskGroup.tasks.models.find((m) => m.name === task.name);
      const volumes = jobTask.volumeMounts.map((volume) => ({
        name: volume.Volume,
        source: taskGroup.volumes[volume.Volume].Source,
      }));

      assert.equal(taskRow.name, task.name, 'Name');
      assert.equal(taskRow.state, task.state, 'State');
      assert.equal(taskRow.message, event.message, 'Event Message');
      assert.equal(
        taskRow.time,
        moment(event.time / 1000000).format("MMM DD, 'YY HH:mm:ss ZZ"),
        'Event Time'
      );

      const expectStats = task.state === 'running';
      assert.equal(taskRow.hasCpuMetrics, expectStats, 'CPU metrics');
      assert.equal(taskRow.hasMemoryMetrics, expectStats, 'Memory metrics');

      const volumesText = taskRow.volumes;
      volumes.forEach((volume) => {
        assert.ok(
          volumesText.includes(volume.name),
          `Found label ${volume.name}`
        );
        assert.ok(
          volumesText.includes(volume.source),
          `Found value ${volume.source}`
        );
      });
    });
  });

  test('each task row should link to the task detail page', async function (assert) {
    const task = server.db.taskStates
      .where({ allocationId: allocation.id })
      .sortBy('name')[0];

    await Allocation.tasks.objectAt(0).clickLink();

    // Make sure the allocation is pending in order to ensure there are no tasks
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task name in task row links to task detail'
    );

    await Allocation.visit({ id: allocation.id });
    await Allocation.tasks.objectAt(0).clickRow();

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task row links to task detail'
    );
  });

  test('tasks with an unhealthy driver have a warning icon', async function (assert) {
    assert.ok(
      Allocation.firstUnhealthyTask().hasUnhealthyDriver,
      'Warning is shown'
    );
  });

  test('proxy task has a proxy tag', async function (assert) {
    // Must create a new job as existing one has loaded and it contains the tasks
    job = server.create('job', {
      groupsCount: 1,
      withGroupServices: true,
      createAllocations: false,
    });

    allocation = server.create('allocation', 'withTaskWithPorts', {
      clientStatus: 'running',
      jobId: job.id,
    });

    const taskState = allocation.taskStates.models.sortBy('name')[0];
    const task = server.schema.tasks.findBy({ name: taskState.name });
    task.update('kind', 'connect-proxy:task');
    task.save();

    await Allocation.visit({ id: allocation.id });

    assert.ok(Allocation.tasks[0].hasProxyTag);
  });

  test('when there are no tasks, an empty state is shown', async function (assert) {
    // Make sure the allocation is pending in order to ensure there are no tasks
    allocation = server.create('allocation', 'withTaskWithPorts', {
      clientStatus: 'pending',
    });
    await Allocation.visit({ id: allocation.id });

    assert.ok(Allocation.isEmpty, 'Task table empty state is shown');
  });

  test('when the allocation has not been rescheduled, the reschedule events section is not rendered', async function (assert) {
    assert.notOk(
      Allocation.hasRescheduleEvents,
      'Reschedule Events section exists'
    );
  });

  test('ports are listed', async function (assert) {
    const allServerPorts = allocation.taskResources.models[0].resources.Ports;

    allServerPorts.sortBy('Label').forEach((serverPort, index) => {
      const renderedPort = Allocation.ports[index];

      assert.equal(renderedPort.name, serverPort.Label);
      assert.equal(renderedPort.to, serverPort.To);
      assert.equal(
        renderedPort.address,
        formatHost(serverPort.HostIP, serverPort.Value)
      );
    });
  });

  test('services are listed', async function (assert) {
    const taskGroup = server.schema.taskGroups.findBy({
      name: allocation.taskGroup,
    });

    assert.equal(Allocation.services.length, taskGroup.services.length);

    taskGroup.services.models.sortBy('name').forEach((serverService, index) => {
      const renderedService = Allocation.services[index];

      assert.equal(renderedService.name, serverService.name);
      assert.equal(renderedService.port, serverService.portLabel);
      assert.equal(renderedService.tags, (serverService.tags || []).join(' '));
    });
  });

  test('when the allocation is not found, an error message is shown, but the URL persists', async function (assert) {
    await Allocation.visit({ id: 'not-a-real-allocation' });

    assert.equal(
      server.pretender.handledRequests
        .filter((request) => !request.url.includes('policy'))
        .findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the nonexistent allocation is made'
    );
    assert.equal(
      currentURL(),
      '/allocations/not-a-real-allocation',
      'The URL persists'
    );
    assert.ok(Allocation.error.isShown, 'Error message is shown');
    assert.equal(
      Allocation.error.title,
      'Not Found',
      'Error message is for 404'
    );
  });

  test('allocation can be stopped', async function (assert) {
    await Allocation.stop.idle();
    await Allocation.stop.confirm();

    assert.equal(
      server.pretender.handledRequests
        .reject((request) => request.url.includes('fuzzy'))
        .findBy('method', 'POST').url,
      `/v1/allocation/${allocation.id}/stop`,
      'Stop request is made for the allocation'
    );
  });

  test('allocation can be restarted', async function (assert) {
    await Allocation.restartAll.idle();
    await Allocation.restart.idle();
    await Allocation.restart.confirm();

    assert.equal(
      server.pretender.handledRequests.findBy('method', 'PUT').url,
      `/v1/client/allocation/${allocation.id}/restart`,
      'Restart request is made for the allocation'
    );

    await Allocation.restart.idle();
    await Allocation.restartAll.idle();
    await Allocation.restartAll.confirm();

    assert.ok(
      server.pretender.handledRequests.filterBy(
        'requestBody',
        JSON.stringify({ AllTasks: true })
      ),
      'Restart all tasks request is made for the allocation'
    );
  });

  test('while an allocation is being restarted, the stop button is disabled', async function (assert) {
    server.pretender.post('/v1/allocation/:id/stop', () => [204, {}, ''], true);

    await Allocation.stop.idle();

    run.later(() => {
      assert.ok(Allocation.stop.isRunning, 'Stop is loading');
      assert.ok(Allocation.restart.isDisabled, 'Restart is disabled');
      assert.ok(Allocation.restartAll.isDisabled, 'Restart All is disabled');
      server.pretender.resolve(server.pretender.requestReferences[0].request);
    }, 500);

    await Allocation.stop.confirm();
  });

  test('if stopping or restarting fails, an error message is shown', async function (assert) {
    server.pretender.post('/v1/allocation/:id/stop', () => [403, {}, '']);

    await Allocation.stop.idle();
    await Allocation.stop.confirm();

    assert.ok(Allocation.inlineError.isShown, 'Inline error is shown');
    assert.ok(
      Allocation.inlineError.title.includes('Could Not Stop Allocation'),
      'Title is descriptive'
    );
    assert.ok(
      /ACL token.+?allocation lifecycle/.test(Allocation.inlineError.message),
      'Message mentions ACLs and the appropriate permission'
    );

    await Allocation.inlineError.dismiss();

    assert.notOk(
      Allocation.inlineError.isShown,
      'Inline error is no longer shown'
    );
  });

  test('when navigating to an allocation, if the allocation no longer exists it does a redirect to previous page', async function (assert) {
    await click('[data-test-breadcrumb="jobs.job.index"]');
    await click('[data-test-tab="allocations"] > a');

    const component = this.owner.lookup('component:allocation-row');
    const router = this.owner.lookup('service:router');
    const allocRoute = this.owner.lookup('route:allocations.allocation');
    const originalMethod = allocRoute.goBackToReferrer;
    allocRoute.goBackToReferrer = () => {
      assert.step('Transition dispatched.');
      router.transitionTo('jobs.job.allocations');
    };

    component.onClick = () =>
      router.transitionTo('allocations.allocation', 'aaa');

    server.get('/allocation/:id', function () {
      return new AdapterError([
        {
          detail: `alloc not found`,
          status: 404,
        },
      ]);
    });

    component.onClick();

    await waitFor('.flash-message.alert-critical');

    assert.verifySteps(['Transition dispatched.']);
    assert
      .dom('.flash-message.alert-critical')
      .exists('A toast error message pops up.');

    // Clean-up
    allocRoute.goBackToReferrer = originalMethod;
  });
});

module('Acceptance | allocation detail (rescheduled)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');

    server.create('node-pool');
    node = server.create('node');
    job = server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'rescheduled');

    await Allocation.visit({ id: allocation.id });
  });

  test('when the allocation has been rescheduled, the reschedule events section is rendered', async function (assert) {
    assert.ok(
      Allocation.hasRescheduleEvents,
      'Reschedule Events section exists'
    );
  });
});

module('Acceptance | allocation detail (not running)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');

    server.create('node-pool');
    node = server.create('node');
    job = server.create('job', { createAllocations: false });
    allocation = server.create('allocation', { clientStatus: 'pending' });

    await Allocation.visit({ id: allocation.id });
  });

  test('when the allocation is not running, the utilization graphs are replaced by an empty message', async function (assert) {
    assert.equal(Allocation.resourceCharts.length, 0, 'No resource charts');
    assert.equal(
      Allocation.resourceEmptyMessage,
      "Allocation isn't running",
      'Empty message is appropriate'
    );
  });

  test('the exec and stop/restart buttons are absent', async function (assert) {
    assert.notOk(Allocation.execButton.isPresent);
    assert.notOk(Allocation.stop.isPresent);
    assert.notOk(Allocation.restart.isPresent);
    assert.notOk(Allocation.restartAll.isPresent);
  });
});

module('Acceptance | allocation detail (preemptions)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('agent');
    server.create('node-pool');
    node = server.create('node');
    job = server.create('job', { createAllocations: false });
  });

  test('shows a dedicated section to the allocation that preempted this allocation', async function (assert) {
    allocation = server.create('allocation', 'preempted');
    const preempter = server.schema.find(
      'allocation',
      allocation.preemptedByAllocation
    );
    const preempterJob = server.schema.find('job', preempter.jobId);
    const preempterClient = server.schema.find('node', preempter.nodeId);

    await Allocation.visit({ id: allocation.id });
    assert.ok(Allocation.wasPreempted, 'Preempted allocation section is shown');
    assert.equal(
      Allocation.preempter.status,
      preempter.clientStatus,
      'Preempter status matches'
    );
    assert.equal(
      Allocation.preempter.name,
      preempter.name,
      'Preempter name matches'
    );
    assert.equal(
      Allocation.preempter.priority,
      preempterJob.priority,
      'Preempter priority matches'
    );

    await Allocation.preempter.visit();
    assert.equal(
      currentURL(),
      `/allocations/${preempter.id}`,
      'Clicking the preempter id navigates to the preempter allocation detail page'
    );

    await Allocation.visit({ id: allocation.id });

    await Allocation.preempter.visitJob();
    assert.equal(
      currentURL(),
      `/jobs/${preempterJob.id}@default`,
      'Clicking the preempter job link navigates to the preempter job page'
    );

    await Allocation.visit({ id: allocation.id });
    await Allocation.preempter.visitClient();
    assert.equal(
      currentURL(),
      `/clients/${preempterClient.id}`,
      'Clicking the preempter client link navigates to the preempter client page'
    );
  });

  test('shows a dedicated section to the allocations this allocation preempted', async function (assert) {
    allocation = server.create('allocation', 'preempter');
    await Allocation.visit({ id: allocation.id });
    assert.ok(
      Allocation.preempted,
      'The allocations this allocation preempted are shown'
    );
  });

  test('each preempted allocation in the table lists basic allocation information', async function (assert) {
    allocation = server.create('allocation', 'preempter');
    await Allocation.visit({ id: allocation.id });

    const preemption = allocation.preemptedAllocations
      .map((id) => server.schema.find('allocation', id))
      .sortBy('modifyIndex')
      .reverse()[0];
    const preemptionRow = Allocation.preemptions.objectAt(0);

    assert.equal(
      Allocation.preemptions.length,
      allocation.preemptedAllocations.length,
      'The preemptions table has a row for each preempted allocation'
    );

    assert.equal(
      preemptionRow.shortId,
      preemption.id.split('-')[0],
      'Preemption short id'
    );
    assert.equal(
      preemptionRow.createTime,
      moment(preemption.createTime / 1000000).format('MMM DD HH:mm:ss ZZ'),
      'Preemption create time'
    );
    assert.equal(
      preemptionRow.modifyTime,
      moment(preemption.modifyTime / 1000000).fromNow(),
      'Preemption modify time'
    );
    assert.equal(
      preemptionRow.status,
      preemption.clientStatus,
      'Client status'
    );
    assert.equal(
      preemptionRow.jobVersion,
      preemption.jobVersion,
      'Job Version'
    );
    assert.equal(
      preemptionRow.client,
      server.db.nodes.find(preemption.nodeId).id.split('-')[0],
      'Node ID'
    );

    await preemptionRow.visitClient();
    assert.equal(
      currentURL(),
      `/clients/${preemption.nodeId}`,
      'Node links to node page'
    );
  });

  test('when an allocation both preempted allocations and was preempted itself, both preemptions sections are shown', async function (assert) {
    allocation = server.create('allocation', 'preempter', 'preempted');
    await Allocation.visit({ id: allocation.id });
    assert.ok(
      Allocation.preempted,
      'The allocations this allocation preempted are shown'
    );
    assert.ok(Allocation.wasPreempted, 'Preempted allocation section is shown');
  });
});

module('Acceptance | allocation detail (services)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    server.create('feature', { name: 'Dynamic Application Sizing' });
    server.createList('agent', 3, 'withConsulLink', 'withVaultLink');
    server.createList('node-pool', 3);
    server.createList('node', 5);
    server.createList('job', 1, { createRecommendations: true });
    const job = server.create('job', {
      withGroupServices: true,
      withTaskServices: true,
      name: 'Service-haver',
      id: 'service-haver',
      namespaceId: 'default',
    });

    const runningAlloc = server.create('allocation', {
      jobId: job.id,
      forceRunningClientStatus: true,
      clientStatus: 'running',
    });
    const otherAlloc = server.db.allocations.reject((j) => j.jobId !== job.id);

    server.db.serviceFragments.update({
      healthChecks: [
        {
          Status: 'success',
          Check: 'check1',
          Timestamp: 99,
          Alloc: runningAlloc.id,
        },
        {
          Status: 'failure',
          Check: 'check2',
          Output: 'One',
          propThatDoesntMatter:
            'this object will be ignored, since it shared a Check name with a later one.',
          Timestamp: 98,
          Alloc: runningAlloc.id,
        },
        {
          Status: 'success',
          Check: 'check2',
          Output: 'Two',
          Timestamp: 99,
          Alloc: runningAlloc.id,
        },
        {
          Status: 'failure',
          Check: 'check3',
          Output: 'Oh no!',
          Timestamp: 99,
          Alloc: runningAlloc.id,
        },
        {
          Status: 'success',
          Check: 'check3',
          Output: 'Wont be seen',
          propThatDoesntMatter:
            'this object will be ignored, in spite of its later timestamp, since it exists on a different alloc',
          Timestamp: 100,
          Alloc: otherAlloc.id,
        },
      ],
    });
  });

  test('Allocation has a list of services with active checks', async function (assert) {
    faker.seed(1);
    const runningAlloc = server.db.allocations.findBy({
      jobId: 'service-haver',
      forceRunningClientStatus: true,
      clientStatus: 'running',
    });
    await Allocation.visit({ id: runningAlloc.id });
    assert.dom('[data-test-service]').exists();
    assert.dom('.service-sidebar').exists();
    assert.dom('.service-sidebar').doesNotHaveClass('open');
    assert
      .dom('[data-test-service-status-bar]')
      .exists('At least one allocation has service health');

    await click('[data-test-service-status-bar]');
    assert.dom('.service-sidebar').hasClass('open');
    assert
      .dom('table.health-checks tr[data-service-health="success"]')
      .exists({ count: 2 }, 'Two successful health checks');
    assert
      .dom('table.health-checks tr[data-service-health="failure"]')
      .exists({ count: 1 }, 'One failing health check');
    assert
      .dom(
        'table.health-checks tr[data-service-health="failure"] td.service-output'
      )
      .containsText('Oh no!');

    await triggerEvent('.page-layout', 'keydown', { key: 'Escape' });
    assert.dom('.service-sidebar').doesNotHaveClass('open');
  });
});
