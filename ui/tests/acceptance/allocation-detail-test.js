import $ from 'jquery';
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
    job = server.create('job', { groupCount: 0 });
    allocation = server.create('allocation', 'withTaskWithPorts', {
      useMessagePassthru: true,
    });

    visit(`/allocations/${allocation.id}`);
  },
});

test('/allocation/:id should name the allocation and link to the corresponding job and node', function(
  assert
) {
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
    event.message,
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

test('when the allocation is not found, an error message is shown, but the URL persists', function(
  assert
) {
  visit('/allocations/not-a-real-allocation');

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/allocation/not-a-real-allocation',
      'A request to the non-existent allocation is made'
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
