import { run } from '@ember/runloop';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import TaskLogs from 'nomad-ui/tests/pages/allocations/task/logs';

let allocation;
let task;

moduleForAcceptance('Acceptance | task logs', {
  beforeEach() {
    server.create('agent');
    server.create('node', 'forceIPv4');
    const job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', { jobId: job.id, clientStatus: 'running' });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    run.later(run, run.cancelTimers, 1000);
    TaskLogs.visit({ id: allocation.id, name: task.name });
  },
});

test('/allocation/:id/:task_name/logs should have a log component', function(assert) {
  assert.equal(currentURL(), `/allocations/${allocation.id}/${task.name}/logs`, 'No redirect');
  assert.ok(TaskLogs.hasTaskLog, 'Task log component found');
});

test('the stdout log immediately starts streaming', function(assert) {
  const node = server.db.nodes.find(allocation.nodeId);
  const logUrlRegex = new RegExp(`${node.httpAddr}/v1/client/fs/logs/${allocation.id}`);
  assert.ok(
    server.pretender.handledRequests.filter(req => logUrlRegex.test(req.url)).length,
    'Log requests were made'
  );
});
