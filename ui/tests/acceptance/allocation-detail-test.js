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
    job = server.create('job', { groupCount: 0 });
    allocation = server.create('allocation', 'withTaskWithPorts');

    visit(`/allocations/${allocation.id}`);
  },
});

test('/allocation/:id should name the allocation and link to the corresponding job and node', function(
  assert
) {
  assert.ok(find('h1').text().includes(allocation.name), 'Allocation name is in the heading');
  assert.ok(find('h3').text().includes(job.name), 'Job name is in the subheading');
  assert.ok(
    find('h3').text().includes(node.id.split('-')[0]),
    'Node short id is in the subheading'
  );

  click('h3 a:eq(0)');
  andThen(() => {
    assert.equal(currentURL(), `/jobs/${job.id}`, 'Job link navigates to the job');
  });

  visit(`/allocations/${allocation.id}`);
  click('h3 a:eq(1)');
  andThen(() => {
    assert.equal(currentURL(), `/nodes/${node.id}`, 'Node link navigates to the node');
  });
});

test('/allocation/:id should list all tasks for the allocation', function(assert) {
  assert.equal(
    find('.tasks tbody tr').length,
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
  const taskRow = find('.tasks tbody tr:eq(0)');
  const events = server.db.taskEvents.where({ taskStateId: task.id });
  const event = events[events.length - 1];

  assert.equal(taskRow.find('td:eq(0)').text().trim(), task.name, 'Name');
  assert.equal(taskRow.find('td:eq(1)').text().trim(), task.state, 'State');
  assert.equal(taskRow.find('td:eq(2)').text().trim(), event.message, 'Event Message');
  assert.equal(
    taskRow.find('td:eq(3)').text().trim(),
    moment(event.time / 1000000).format('DD/MM/YY HH:mm:ss [UTC]'),
    'Event Time'
  );

  assert.ok(reservedPorts.length, 'The task has reserved ports');

  const addressesText = taskRow.find('td:eq(4)').text();
  reservedPorts.forEach(port => {
    assert.ok(addressesText.includes(port.Label), `Found label ${port.Label}`);
    assert.ok(addressesText.includes(port.Value), `Found value ${port.Value}`);
  });
});

test('/allocation/:id should list recent events for each task', function(assert) {
  const tasks = server.db.taskStates.where({ allocationId: allocation.id });
  assert.equal(
    find('.task-state-events').length,
    tasks.length,
    'A task state event block per task'
  );
});

test('each recent events list should include the name, state, and time info for the task', function(
  assert
) {
  const task = server.db.taskStates.where({ allocationId: allocation.id })[0];
  const recentEventsSection = find('.task-state-events:eq(0)');
  const heading = recentEventsSection.find('.message-header').text().trim();

  assert.ok(heading.includes(task.name), 'Task name');
  assert.ok(heading.includes(task.state), 'Task state');
  assert.ok(
    heading.includes(moment(task.startedAt).format('DD/MM/YY HH:mm:ss [UTC]')),
    'Task started at'
  );
});

test('each recent events list should list all recent events for the task', function(assert) {
  const task = server.db.taskStates.where({ allocationId: allocation.id })[0];
  const events = server.db.taskEvents.where({ taskStateId: task.id });

  assert.equal(
    find('.task-state-events:eq(0) .task-events tbody tr').length,
    events.length,
    `Lists ${events.length} events`
  );
});

test('each recent event should list the time, type, and description of the event', function(
  assert
) {
  const task = server.db.taskStates.where({ allocationId: allocation.id })[0];
  const event = server.db.taskEvents.where({ taskStateId: task.id })[0];
  const recentEvent = find('.task-state-events:eq(0) .task-events tbody tr:last');

  assert.equal(
    recentEvent.find('td:eq(0)').text().trim(),
    moment(event.time / 1000000).format('DD/MM/YY HH:mm:ss [UTC]'),
    'Event timestamp'
  );
  assert.equal(recentEvent.find('td:eq(1)').text().trim(), event.type, 'Event type');
  assert.equal(recentEvent.find('td:eq(2)').text().trim(), event.message, 'Event message');
});
