/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { module, skip, test } from 'qunit';
import { currentURL, settled } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import Service from '@ember/service';
import Exec from 'nomad-ui/tests/pages/exec';
import KEYS from 'nomad-ui/utils/keys';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

module('Acceptance | exec', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    window.localStorage.clear();
    window.sessionStorage.clear();

    faker.seed(1);

    server.create('agent');
    server.create('node-pool');
    server.create('node');

    this.job = server.create('job', {
      groupsCount: 2,
      groupTaskCount: 5,
      createAllocations: false,
      status: 'running',
    });

    this.job.taskGroups.models.forEach((taskGroup) => {
      const alloc = server.create('allocation', {
        jobId: this.job.id,
        taskGroup: taskGroup.name,
        forceRunningClientStatus: true,
      });
      server.db.taskStates.update(
        { allocationId: alloc.id },
        { state: 'running' }
      );
    });
  });

  test('it passes an accessibility audit', async function (assert) {
    await Exec.visitJob({ job: this.job.id });
    await a11yAudit(assert);
  });

  test('/exec/:job should show the region, namespace, and job name', async function (assert) {
    server.create('namespace');
    let namespace = server.create('namespace');

    server.create('region', { id: 'global' });
    server.create('region', { id: 'region-2' });

    this.job = server.create('job', {
      createAllocations: false,
      namespaceId: namespace.id,
      status: 'running',
    });

    await Exec.visitJob({
      job: this.job.id,
      namespace: namespace.id,
      region: 'region-2',
    });

    assert.ok(document.title.includes('Exec - region-2'));

    assert.equal(Exec.header.region.text, this.job.region);
    assert.equal(Exec.header.namespace.text, this.job.namespace);
    assert.equal(Exec.header.job, this.job.name);

    assert.notOk(Exec.jobDead.isPresent);
  });

  test('/exec/:job should not show region and namespace when there are none', async function (assert) {
    await Exec.visitJob({ job: this.job.id });

    assert.ok(Exec.header.region.isHidden);
    assert.ok(Exec.header.namespace.isHidden);
  });

  test('/exec/:job should show the task groups collapsed by default and allow the tasks to be shown', async function (assert) {
    const firstTaskGroup = this.job.taskGroups.models.sortBy('name')[0];
    await Exec.visitJob({ job: this.job.id });

    assert.equal(Exec.taskGroups.length, this.job.taskGroups.length);

    assert.equal(Exec.taskGroups[0].name, firstTaskGroup.name);
    assert.equal(Exec.taskGroups[0].tasks.length, 0);
    assert.ok(Exec.taskGroups[0].chevron.isRight);
    assert.notOk(Exec.taskGroups[0].isLoading);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, firstTaskGroup.tasks.length);
    assert.notOk(Exec.taskGroups[0].tasks[0].isActive);
    assert.ok(Exec.taskGroups[0].chevron.isDown);

    await percySnapshot(assert);

    await Exec.taskGroups[0].click();
    assert.equal(Exec.taskGroups[0].tasks.length, 0);
  });

  test('/exec/:job should require selecting a task', async function (assert) {
    await Exec.visitJob({ job: this.job.id });

    assert.equal(
      window.execTerminal.buffer.active.getLine(0).translateToString().trim(),
      'Select a task to start your session.'
    );
  });

  test('a task group with a pending allocation shows a loading spinner', async function (assert) {
    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    this.server.db.allocations.update(
      { taskGroup: taskGroup.name },
      { clientStatus: 'pending' }
    );

    await Exec.visitJob({ job: this.job.id });
    assert.ok(Exec.taskGroups[0].isLoading);
  });

  test('a task group with no running task states or pending allocations should not be shown', async function (assert) {
    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    this.server.db.allocations.update(
      { taskGroup: taskGroup.name },
      { clientStatus: 'failed' }
    );

    await Exec.visitJob({ job: this.job.id });
    assert.notEqual(Exec.taskGroups[0].name, taskGroup.name);
  });

  test('an inactive task should not be shown', async function (assert) {
    let notRunningTaskGroup = this.job.taskGroups.models.sortBy('name')[0];
    this.server.db.allocations.update(
      { taskGroup: notRunningTaskGroup.name },
      { clientStatus: 'failed' }
    );

    let runningTaskGroup = this.job.taskGroups.models.sortBy('name')[1];
    runningTaskGroup.tasks.models.forEach((task, index) => {
      let state = 'running';
      if (index > 0) {
        state = 'dead';
      }
      this.server.db.taskStates.update({ name: task.name }, { state });
    });

    await Exec.visitJob({ job: this.job.id });
    await Exec.taskGroups[0].click();

    assert.equal(Exec.taskGroups[0].tasks.length, 1);
  });

  test('a task that becomes active should appear', async function (assert) {
    let notRunningTaskGroup = this.job.taskGroups.models.sortBy('name')[0];
    this.server.db.allocations.update(
      { taskGroup: notRunningTaskGroup.name },
      { clientStatus: 'failed' }
    );

    let runningTaskGroup = this.job.taskGroups.models.sortBy('name')[1];
    let changingTaskStateName;
    runningTaskGroup.tasks.models.sortBy('name').forEach((task, index) => {
      let state = 'running';
      if (index > 0) {
        state = 'dead';
      }
      this.server.db.taskStates.update({ name: task.name }, { state });

      if (index === 1) {
        changingTaskStateName = task.name;
      }
    });

    await Exec.visitJob({ job: this.job.id });
    await Exec.taskGroups[0].click();

    assert.equal(Exec.taskGroups[0].tasks.length, 1);

    // Approximate new task arrival via polling by changing a finished task state to be not finished
    this.owner
      .lookup('service:store')
      .peekAll('allocation')
      .forEach((allocation) => {
        const changingTaskState = allocation.states.findBy(
          'name',
          changingTaskStateName
        );

        if (changingTaskState) {
          changingTaskState.set('state', 'running');
        }
      });

    await settled();

    assert.equal(Exec.taskGroups[0].tasks.length, 2);
    assert.equal(Exec.taskGroups[0].tasks[1].name, changingTaskStateName);
  });

  test('a dead job has an inert window', async function (assert) {
    this.job.status = 'dead';
    this.job.save();

    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];

    this.server.db.taskStates.update({ finishedAt: new Date() });

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
    });

    assert.ok(Exec.jobDead.isPresent);
    assert.equal(
      Exec.jobDead.message,
      `Job ${this.job.name} is dead and cannot host an exec session.`
    );
  });

  test('when a job dies the exec window becomes inert', async function (assert) {
    await Exec.visitJob({ job: this.job.id });

    // Approximate live-polling job death
    this.owner
      .lookup('service:store')
      .peekAll('job')
      .forEach((job) => job.set('status', 'dead'));

    await settled();

    assert.ok(Exec.jobDead.isPresent);
  });

  test('visiting a path with a task group should open the group by default', async function (assert) {
    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    await Exec.visitTaskGroup({ job: this.job.id, task_group: taskGroup.name });

    assert.equal(Exec.taskGroups[0].tasks.length, taskGroup.tasks.length);
    assert.ok(Exec.taskGroups[0].chevron.isDown);

    let task = taskGroup.tasks.models.sortBy('name')[0];
    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
    });

    assert.equal(Exec.taskGroups[0].tasks.length, taskGroup.tasks.length);
    assert.ok(Exec.taskGroups[0].chevron.isDown);
  });

  test('navigating to a task adds its name to the route, chooses an allocation, and assigns a default command', async function (assert) {
    await Exec.visitJob({ job: this.job.id });
    await Exec.taskGroups[0].click();
    await Exec.taskGroups[0].tasks[0].click();

    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];

    let taskStates = this.server.db.taskStates.where({
      name: task.name,
    });
    let allocationId = taskStates.find((ts) => ts.allocationId).allocationId;

    await settled();

    assert.equal(
      currentURL(),
      `/exec/${this.job.id}/${taskGroup.name}/${task.name}`
    );
    assert.ok(Exec.taskGroups[0].tasks[0].isActive);

    assert.equal(
      window.execTerminal.buffer.active.getLine(2).translateToString().trim(),
      'Multiple instances of this task are running. The allocation below was selected by random draw.'
    );

    assert.equal(
      window.execTerminal.buffer.active.getLine(4).translateToString().trim(),
      'Customize your command, then hit ‘return’ to run.'
    );

    assert.equal(
      window.execTerminal.buffer.active.getLine(6).translateToString().trim(),
      `$ nomad alloc exec -i -t -task ${task.name} ${
        allocationId.split('-')[0]
      } /bin/bash`
    );

    const terminalTextRendered = assert.async();
    setTimeout(async () => {
      await percySnapshot(assert);
      terminalTextRendered();
    }, 1000);
  });

  test('an allocation can be specified', async function (assert) {
    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];
    let allocations = this.server.db.allocations.where({
      jobId: this.job.id,
      taskGroup: taskGroup.name,
    });
    let allocation = allocations[allocations.length - 1];

    this.server.db.taskStates.update(
      { name: task.name },
      { name: 'spaced name!' }
    );

    task.name = 'spaced name!';
    task.save();

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
      allocation: allocation.id.split('-')[0],
    });

    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(4).translateToString().trim(),
      `$ nomad alloc exec -i -t -task spaced\\ name\\! ${
        allocation.id.split('-')[0]
      } /bin/bash`
    );
  });

  test('running the command opens the socket for reading/writing and detects it closing', async function (assert) {
    let mockSocket = new MockSocket();
    let mockSockets = Service.extend({
      getTaskStateSocket(taskState, command) {
        assert.equal(taskState.name, task.name);
        assert.equal(taskState.allocation.id, allocation.id);

        assert.equal(command, '/bin/bash');

        assert.step('Socket built');

        return mockSocket;
      },
    });

    this.owner.register('service:sockets', mockSockets);

    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];
    let allocations = this.server.db.allocations.where({
      jobId: this.job.id,
      taskGroup: taskGroup.name,
    });
    let allocation = allocations[allocations.length - 1];

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
      allocation: allocation.id.split('-')[0],
    });

    await settled();

    await Exec.terminal.pressEnter();
    await settled();
    mockSocket.onopen();

    assert.verifySteps(['Socket built']);

    mockSocket.onmessage({
      data: '{"stdout":{"data":"c2gtMy4yIPCfpbMk"}}',
    });

    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(5).translateToString().trim(),
      'sh-3.2 🥳$'
    );

    await Exec.terminal.pressEnter();
    await settled();

    assert.deepEqual(mockSocket.sent, [
      '{"version":1,"auth_token":""}',
      `{"tty_size":{"width":${window.execTerminal.cols},"height":${window.execTerminal.rows}}}`,
      '{"stdin":{"data":"DQ=="}}',
    ]);

    await mockSocket.onclose();
    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(6).translateToString().trim(),
      'The connection has closed.'
    );
  });

  test('the opening message includes the token if it exists', async function (assert) {
    const { secretId } = server.create('token');
    window.localStorage.nomadTokenSecret = secretId;

    let mockSocket = new MockSocket();
    let mockSockets = Service.extend({
      getTaskStateSocket() {
        return mockSocket;
      },
    });

    this.owner.register('service:sockets', mockSockets);

    let taskGroup = this.job.taskGroups.models[0];
    let task = taskGroup.tasks.models[0];
    let allocations = this.server.db.allocations.where({
      jobId: this.job.id,
      taskGroup: taskGroup.name,
    });
    let allocation = allocations[allocations.length - 1];

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
      allocation: allocation.id.split('-')[0],
    });

    await Exec.terminal.pressEnter();
    await settled();
    mockSocket.onopen();

    await Exec.terminal.pressEnter();
    await settled();

    assert.equal(
      mockSocket.sent[0],
      `{"version":1,"auth_token":"${secretId}"}`
    );
  });

  test('only one socket is opened after switching between tasks', async function (assert) {
    let mockSockets = Service.extend({
      getTaskStateSocket() {
        assert.step('Socket built');
        return new MockSocket();
      },
    });

    this.owner.register('service:sockets', mockSockets);

    await Exec.visitJob({
      job: this.job.id,
    });

    await settled();

    await Exec.taskGroups[0].click();
    await Exec.taskGroups[0].tasks[0].click();

    await Exec.taskGroups[1].click();
    await Exec.taskGroups[1].tasks[0].click();

    await Exec.terminal.pressEnter();

    assert.verifySteps(['Socket built']);
  });

  test('the command can be customised', async function (assert) {
    let mockSockets = Service.extend({
      getTaskStateSocket(taskState, command) {
        assert.equal(command, '/sh');
        window.localStorage.getItem('nomadExecCommand', JSON.stringify('/sh'));

        assert.step('Socket built');

        return new MockSocket();
      },
    });

    this.owner.register('service:sockets', mockSockets);

    await Exec.visitJob({ job: this.job.id });
    await Exec.taskGroups[0].click();
    await Exec.taskGroups[0].tasks[0].click();

    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];
    let allocation = this.server.db.allocations.findBy({
      jobId: this.job.id,
      taskGroup: taskGroup.name,
    });

    await settled();

    // Delete /bash
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);

    // Delete /bin and try to go beyond
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);
    await window.execTerminal.simulateCommandDataEvent(KEYS.DELETE);

    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(6).translateToString().trim(),
      `$ nomad alloc exec -i -t -task ${task.name} ${
        allocation.id.split('-')[0]
      }`
    );

    await window.execTerminal.simulateCommandDataEvent('/sh');

    await Exec.terminal.pressEnter();
    await settled();

    assert.verifySteps(['Socket built']);
  });

  test('a persisted customised command is recalled', async function (assert) {
    window.localStorage.setItem('nomadExecCommand', JSON.stringify('/bin/sh'));

    let taskGroup = this.job.taskGroups.models[0];
    let task = taskGroup.tasks.models[0];
    let allocations = this.server.db.allocations.where({
      jobId: this.job.id,
      taskGroup: taskGroup.name,
    });
    let allocation = allocations[allocations.length - 1];

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
      allocation: allocation.id.split('-')[0],
    });

    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(4).translateToString().trim(),
      `$ nomad alloc exec -i -t -task ${task.name} ${
        allocation.id.split('-')[0]
      } /bin/sh`
    );
  });

  skip('when a task state finishes submitting a command displays an error', async function (assert) {
    let taskGroup = this.job.taskGroups.models.sortBy('name')[0];
    let task = taskGroup.tasks.models.sortBy('name')[0];

    await Exec.visitTask({
      job: this.job.id,
      task_group: taskGroup.name,
      task_name: task.name,
    });

    // Approximate allocation failure via polling
    this.owner
      .lookup('service:store')
      .peekAll('allocation')
      .forEach((allocation) => allocation.set('clientStatus', 'failed'));

    await Exec.terminal.pressEnter();
    await settled();

    assert.equal(
      window.execTerminal.buffer.active.getLine(7).translateToString().trim(),
      `Failed to open a socket because task ${task.name} is not active.`
    );
  });
});

class MockSocket {
  constructor() {
    this.sent = [];
  }

  send(message) {
    this.sent.push(message);
  }
}
