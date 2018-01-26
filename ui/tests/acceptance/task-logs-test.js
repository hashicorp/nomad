import { run } from '@ember/runloop';
import { find } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let allocation;
let task;

moduleForAcceptance('Acceptance | task logs', {
  beforeEach() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job');

    allocation = server.db.allocations.where({ jobId: job.id })[0];
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    run.later(run, run.cancelTimers, 1000);
    visit(`/allocations/${allocation.id}/${task.name}/logs`);
  },
});

test('/allocation/:id/:task_name/logs should have a log component', function(assert) {
  assert.equal(currentURL(), `/allocations/${allocation.id}/${task.name}/logs`, 'No redirect');
  assert.ok(find('[data-test-task-log]'), 'Task log component found');
});

test('the stdout log immediately starts streaming', function(assert) {
  const node = server.db.nodes.find(allocation.nodeId);
  const logUrlRegex = new RegExp(`${node.httpAddr}/v1/client/fs/logs/${allocation.id}`);
  assert.ok(
    server.pretender.handledRequests.filter(req => logUrlRegex.test(req.url)).length,
    'Log requests were made'
  );
});
