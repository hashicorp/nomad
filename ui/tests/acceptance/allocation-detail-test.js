import { assign } from '@ember/polyfills';
import { currentURL } from 'ember-native-dom-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';
import moment from 'moment';

let job;
let node;
let allocation;

module('Acceptance | allocation detail', function(hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function() {
    server.create('agent');

    node = server.create('node');
    job = server.create('job', { groupsCount: 1, createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'running' });

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

    Allocation.visit({ id: allocation.id });
  });

  test('/allocation/:id should name the allocation and link to the corresponding job and node', function(assert) {
    assert.ok(Allocation.title.includes(allocation.name), 'Allocation name is in the heading');
    assert.equal(Allocation.details.job, job.name, 'Job name is in the subheading');
    assert.equal(
      Allocation.details.client,
      node.id.split('-')[0],
      'Node short id is in the subheading'
    );

    Allocation.details.visitJob();

    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job link navigates to the job');

    Allocation.visit({ id: allocation.id });

    Allocation.details.visitClient();

    assert.equal(currentURL(), `/clients/${node.id}`, 'Client link navigates to the client');
  });

  test('/allocation/:id should include resource utilization graphs', function(assert) {
    assert.equal(Allocation.resourceCharts.length, 2, 'Two resource utilization graphs');
    assert.equal(Allocation.resourceCharts.objectAt(0).name, 'CPU', 'First chart is CPU');
    assert.equal(Allocation.resourceCharts.objectAt(1).name, 'Memory', 'Second chart is Memory');
  });

  test('/allocation/:id should list all tasks for the allocation', function(assert) {
    assert.equal(
      Allocation.tasks.length,
      server.db.taskStates.where({ allocationId: allocation.id }).length,
      'Table lists all tasks'
    );
    assert.notOk(Allocation.isEmpty, 'Task table empty state is not shown');
  });

  test('each task row should list high-level information for the task', function(assert) {
    const task = server.db.taskStates.where({ allocationId: allocation.id }).sortBy('name')[0];
    const taskResources = allocation.taskResourcesIds
      .map(id => server.db.taskResources.find(id))
      .sortBy('name')[0];
    const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
    const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
    const taskRow = Allocation.tasks.objectAt(0);
    const events = server.db.taskEvents.where({ taskStateId: task.id });
    const event = events[events.length - 1];

    assert.equal(taskRow.name, task.name, 'Name');
    assert.equal(taskRow.state, task.state, 'State');
    assert.equal(taskRow.message, event.displayMessage, 'Event Message');
    assert.equal(
      taskRow.time,
      moment(event.time / 1000000).format("MMM DD, 'YY HH:mm:ss ZZ"),
      'Event Time'
    );

    assert.ok(reservedPorts.length, 'The task has reserved ports');
    assert.ok(dynamicPorts.length, 'The task has dynamic ports');

    const addressesText = taskRow.ports;
    reservedPorts.forEach(port => {
      assert.ok(addressesText.includes(port.Label), `Found label ${port.Label}`);
      assert.ok(addressesText.includes(port.Value), `Found value ${port.Value}`);
    });
    dynamicPorts.forEach(port => {
      assert.ok(addressesText.includes(port.Label), `Found label ${port.Label}`);
      assert.ok(addressesText.includes(port.Value), `Found value ${port.Value}`);
    });
  });

  test('each task row should link to the task detail page', function(assert) {
    const task = server.db.taskStates.where({ allocationId: allocation.id }).sortBy('name')[0];

    Allocation.tasks.objectAt(0).clickLink();

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task name in task row links to task detail'
    );

    Allocation.visit({ id: allocation.id });

    Allocation.tasks.objectAt(0).clickRow();

    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task row links to task detail'
    );
  });

  test('tasks with an unhealthy driver have a warning icon', function(assert) {
    assert.ok(Allocation.firstUnhealthyTask().hasUnhealthyDriver, 'Warning is shown');
  });

  test('when there are no tasks, an empty state is shown', function(assert) {
    // Make sure the allocation is pending in order to ensure there are no tasks
    allocation = server.create('allocation', 'withTaskWithPorts', { clientStatus: 'pending' });
    Allocation.visit({ id: allocation.id });

    assert.ok(Allocation.isEmpty, 'Task table empty state is shown');
  });

  test('when the allocation has not been rescheduled, the reschedule events section is not rendered', function(assert) {
    assert.notOk(Allocation.hasRescheduleEvents, 'Reschedule Events section exists');
  });

  test('when the allocation is not found, an error message is shown, but the URL persists', function(assert) {
    Allocation.visit({ id: 'not-a-real-allocation' });

    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the nonexistent allocation is made'
    );
    assert.equal(currentURL(), '/allocations/not-a-real-allocation', 'The URL persists');
    assert.ok(Allocation.error.isShown, 'Error message is shown');
    assert.equal(Allocation.error.title, 'Not Found', 'Error message is for 404');
  });
});

module('Acceptance | allocation detail (rescheduled)', function(hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function() {
    server.create('agent');

    node = server.create('node');
    job = server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'rescheduled');

    Allocation.visit({ id: allocation.id });
  });

  test('when the allocation has been rescheduled, the reschedule events section is rendered', function(assert) {
    assert.ok(Allocation.hasRescheduleEvents, 'Reschedule Events section exists');
  });
});

module('Acceptance | allocation detail (not running)', function(hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function() {
    server.create('agent');

    node = server.create('node');
    job = server.create('job', { createAllocations: false });
    allocation = server.create('allocation', { clientStatus: 'pending' });

    Allocation.visit({ id: allocation.id });
  });

  test('when the allocation is not running, the utilization graphs are replaced by an empty message', function(assert) {
    assert.equal(Allocation.resourceCharts.length, 0, 'No resource charts');
    assert.equal(
      Allocation.resourceEmptyMessage,
      "Allocation isn't running",
      'Empty message is appropriate'
    );
  });
});
