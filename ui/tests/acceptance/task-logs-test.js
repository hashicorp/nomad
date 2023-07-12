/* eslint-disable qunit/require-expect */
import { click, currentURL, findAll } from '@ember/test-helpers';
import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import TaskLogs from 'nomad-ui/tests/pages/allocations/task/logs';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

let allocation;
let task;
let job;

module('Acceptance | task logs', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    faker.seed(1);
    server.create('agent');
    server.create('node', 'forceIPv4');
    job = server.create('job', { createAllocations: false });

    allocation = server.create('allocation', {
      jobId: job.id,
      clientStatus: 'running',
    });
    task = server.db.taskStates.where({ allocationId: allocation.id })[0];

    run.later(run, run.cancelTimers, 1000);
  });

  test('it passes an accessibility audit', async function (assert) {
    await TaskLogs.visit({ id: allocation.id, name: task.name });
    await a11yAudit(assert);
    await percySnapshot(assert);
  });

  test('/allocation/:id/:task_name/logs should have a log component', async function (assert) {
    await TaskLogs.visit({ id: allocation.id, name: task.name });
    assert.equal(
      currentURL(),
      `/allocations/${allocation.id}/${task.name}/logs`,
      'No redirect'
    );
    assert.ok(TaskLogs.hasTaskLog, 'Task log component found');
    assert.equal(document.title, `Task ${task.name} logs - Nomad`);
  });

  test('the stdout log immediately starts streaming', async function (assert) {
    await TaskLogs.visit({ id: allocation.id, name: task.name });
    const node = server.db.nodes.find(allocation.nodeId);
    const logUrlRegex = new RegExp(
      `${node.httpAddr}/v1/client/fs/logs/${allocation.id}`
    );
    assert.ok(
      server.pretender.handledRequests.filter((req) =>
        logUrlRegex.test(req.url)
      ).length,
      'Log requests were made'
    );
  });

  test('logs are accessible in a sidebar context', async function (assert) {
    await TaskLogs.visitParentJob({
      id: job.id,
      allocationId: allocation.id,
      name: task.name,
    });
    assert.notOk(TaskLogs.sidebarIsPresent, 'Sidebar is not present');

    run.later(() => {
      run.cancelTimers();
    }, 500);

    const taskRow = [
      ...findAll('.task-sub-row').filter((row) => {
        return row.textContent.includes(task.name);
      }),
    ][0];

    await click(taskRow.querySelector('button.logs-sidebar-trigger'));

    assert.ok(TaskLogs.sidebarIsPresent, 'Sidebar is present');
    assert
      .dom('.task-context-sidebar h1.title')
      .includesText(task.name, 'Sidebar title is correct');
    assert
      .dom('.task-context-sidebar h1.title')
      .includesText(task.state, 'Task state is correctly displayed');
    await percySnapshot(assert, {
      percyCSS: `
        .allocation-row td { display: none; }
        .task-events table td:nth-child(1) { color: transparent; }
      `,
    });

    await click('.sidebar button.close');
    assert.notOk(TaskLogs.sidebarIsPresent, 'Sidebar is not present');
  });
});
