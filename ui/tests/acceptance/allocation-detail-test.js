import $ from 'jquery';
import { assign } from '@ember/polyfills';
import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moment from 'moment';

let job;
let node;
let allocation;

moduleForAcceptance('Acceptance | allocation detail', {
  beforeEach() {
    server.create('agent');

    node = server.create('node');
    job = server.create('job', { groupCount: 0, createAllocations: false });
    allocation = server.create('allocation', 'withTaskWithPorts');

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

    visit(`/allocations/${allocation.id}`);
  },
});

test('/allocation/:id should name the allocation and link to the corresponding job and node', function(assert) {
  assert.ok(
    find('[data-test-title]').textContent.includes(allocation.name),
    'Allocation name is in the heading'
  );
  assert.equal(
    find('[data-test-allocation-details] [data-test-job-link]').textContent.trim(),
    job.name,
    'Job name is in the subheading'
  );
  assert.equal(
    find('[data-test-allocation-details] [data-test-client-link]').textContent.trim(),
    node.id.split('-')[0],
    'Node short id is in the subheading'
  );

  andThen(() => {
    click('[data-test-allocation-details] [data-test-job-link]');
  });

  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job link navigates to the job');
  });

  visit(`/allocations/${allocation.id}`);

  andThen(() => {
    click('[data-test-allocation-details] [data-test-client-link]');
  });

  andThen(() => {
    assert.equal(currentURL(), `/clients/${node.id}`, 'Client link navigates to the client');
  });
});

test('/allocation/:id should list all tasks for the allocation', function(assert) {
  assert.equal(
    findAll('[data-test-task-row]').length,
    server.db.taskStates.where({ allocationId: allocation.id }).length,
    'Table lists all tasks'
  );
});

test('each task row should list high-level information for the task', function(assert) {
  const task = server.db.taskStates.where({ allocationId: allocation.id }).sortBy('name')[0];
  const taskResources = allocation.taskResourcesIds
    .map(id => server.db.taskResources.find(id))
    .sortBy('name')[0];
  const reservedPorts = taskResources.resources.Networks[0].ReservedPorts;
  const dynamicPorts = taskResources.resources.Networks[0].DynamicPorts;
  const taskRow = $(findAll('[data-test-task-row]')[0]);
  const events = server.db.taskEvents.where({ taskStateId: task.id });
  const event = events[events.length - 1];

  assert.equal(
    taskRow
      .find('[data-test-name]')
      .text()
      .trim(),
    task.name,
    'Name'
  );
  assert.equal(
    taskRow
      .find('[data-test-state]')
      .text()
      .trim(),
    task.state,
    'State'
  );
  assert.equal(
    taskRow
      .find('[data-test-message]')
      .text()
      .trim(),
    event.displayMessage,
    'Event Message'
  );
  assert.equal(
    taskRow
      .find('[data-test-time]')
      .text()
      .trim(),
    moment(event.time / 1000000).format('MM/DD/YY HH:mm:ss'),
    'Event Time'
  );

  assert.ok(reservedPorts.length, 'The task has reserved ports');
  assert.ok(dynamicPorts.length, 'The task has dynamic ports');

  const addressesText = taskRow.find('[data-test-ports]').text();
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

  click('[data-test-task-row] [data-test-name] a');

  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task name in task row links to task detail'
    );
  });

  andThen(() => {
    visit(`/allocations/${allocation.id}`);
  });

  andThen(() => {
    click('[data-test-task-row]');
  });

  andThen(() => {
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}`,
      'Task row links to task detail'
    );
  });
});

test('tasks with an unhealthy driver have a warning icon', function(assert) {
  assert.ok(find('[data-test-task-row] [data-test-icon="unhealthy-driver"]'), 'Warning is shown');
});

test('when the allocation has not been rescheduled, the reschedule events section is not rendered', function(assert) {
  assert.notOk(find('[data-test-reschedule-events]'), 'Reschedule Events section exists');
});

test('when the allocation is not found, an error message is shown, but the URL persists', function(assert) {
  visit('/allocations/not-a-real-allocation');

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the nonexistent allocation is made'
    );
    assert.equal(currentURL(), '/allocations/not-a-real-allocation', 'The URL persists');
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});

moduleForAcceptance('Acceptance | allocation detail (rescheduled)', {
  beforeEach() {
    server.create('agent');

    node = server.create('node');
    job = server.create('job', { createAllocations: false });
    allocation = server.create('allocation', 'rescheduled');

    visit(`/allocations/${allocation.id}`);
  },
});

test('when the allocation has been rescheduled, the reschedule events section is rendered', function(assert) {
  assert.ok(find('[data-test-reschedule-events]'), 'Reschedule Events section exists');
});
