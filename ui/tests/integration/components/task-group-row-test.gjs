/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find, render, settled } from '@ember/test-helpers';
import { startMirage } from 'nomad-ui/tests/helpers/start-mirage';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import TaskGroupRow from 'nomad-ui/components/task-group-row';

const jobName = 'test-job';
const jobId = JSON.stringify([jobName, 'default']);

let managementToken;
let clientToken;

const makeJob = (server, props = {}) => {
  server.create('namespace', {
    id: 'default',
  });
  const job = server.create('job', {
    id: jobName,
    groupCount: 0,
    createAllocations: false,
    shallow: true,
    ...props,
  });
  const noScalingGroup = server.create('task-group', {
    job,
    name: 'no-scaling',
    shallow: true,
    withScaling: false,
  });
  const scalingGroup = server.create('task-group', {
    job,
    count: 2,
    name: 'scaling',
    shallow: true,
    withScaling: true,
  });
  job.update({
    taskGroupIds: [noScalingGroup.id, scalingGroup.id],
  });
};

module('Integration | Component | task group row', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(async function () {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.token = this.owner.lookup('service:token');
    this.server = startMirage();
    this.server.create('node-pool');
    this.server.create('node');

    managementToken = this.server.create('token');
    clientToken = this.server.create('token');
    window.localStorage.nomadTokenSecret = managementToken.secretId;
  });

  hooks.afterEach(function () {
    this.server.shutdown();
    window.localStorage.clear();
  });

  const renderComponent = (context) =>
    render(<template><TaskGroupRow @taskGroup={{context.group}} /></template>);

  test('Task group row conditionally shows scaling buttons based on the presence of the scaling attr on the task group', async function (assert) {
    makeJob(this.server, { noActiveDeployment: true });
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    this.set('group', job.taskGroups.find(item => get(item, 'name') === 'no-scaling'));

    await renderComponent(this);
    assert.notOk(find('[data-test-scale]'));

    this.set('group', job.taskGroups.find(item => get(item, 'name') === 'scaling'));

    await settled();
    assert.ok(find('[data-test-scale]'));

    await componentA11yAudit(this.element, assert);
  });

  test('Clicking scaling buttons immediately updates the rendered count but debounces the scaling API request', async function (assert) {
    makeJob(this.server, { noActiveDeployment: true });
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    this.set('group', job.taskGroups.find(item => get(item, 'name') === 'scaling'));

    await renderComponent(this);
    assert.strictEqual(
      Number(find('[data-test-task-group-count]').textContent.trim()),
      2,
    );

    find('[data-test-scale="increment"]').click();
    assert.strictEqual(
      Number(find('[data-test-task-group-count]').textContent.trim()),
      3,
    );

    find('[data-test-scale="increment"]').click();
    assert.strictEqual(
      Number(find('[data-test-task-group-count]').textContent.trim()),
      4,
    );

    assert.notOk(
      this.server.pretender.handledRequests.find(
        (req) => req.method === 'POST' && req.url.endsWith('/scale'),
      ),
    );

    await settled();
    const scaleRequests = this.server.pretender.handledRequests.filter(
      (req) => req.method === 'POST' && req.url.endsWith('/scale'),
    );
    assert.strictEqual(scaleRequests.length, 1);
    assert.strictEqual(JSON.parse(scaleRequests[0].requestBody).Count, 4);
  });

  test('When the current count is equal to the max count, the increment count button is disabled', async function (assert) {
    makeJob(this.server, { noActiveDeployment: true });
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    const group = job.taskGroups.find(item => get(item, 'name') === 'scaling');
    group.set('count', group.scaling.max);
    this.set('group', group);

    await renderComponent(this);
    assert.ok(find('[data-test-scale="increment"]:disabled'));

    await componentA11yAudit(this.element, assert);
  });

  test('When the current count is equal to the min count, the decrement count button is disabled', async function (assert) {
    makeJob(this.server, { noActiveDeployment: true });
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    const group = job.taskGroups.find(item => get(item, 'name') === 'scaling');
    group.set('count', group.scaling.min);
    this.set('group', group);

    await renderComponent(this);
    assert.ok(find('[data-test-scale="decrement"]:disabled'));

    await componentA11yAudit(this.element, assert);
  });

  test('When there is an active deployment, both scale buttons are disabled', async function (assert) {
    makeJob(this.server, { activeDeployment: true });
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    this.set('group', job.taskGroups.find(item => get(item, 'name') === 'scaling'));

    await renderComponent(this);
    assert.ok(find('[data-test-scale="increment"]:disabled'));
    assert.ok(find('[data-test-scale="decrement"]:disabled'));

    await componentA11yAudit(this.element, assert);
  });

  test('When the current ACL token does not have the namespace:scale-job or namespace:submit-job policy rule', async function (assert) {
    makeJob(this.server, { noActiveDeployment: true });
    window.localStorage.nomadTokenSecret = clientToken.secretId;
    this.token.fetchSelfTokenAndPolicies.perform();
    await settled();

    const job = await this.store.find('job', jobId);
    this.set('group', job.taskGroups.find(item => get(item, 'name') === 'scaling'));

    await renderComponent(this);
    assert.ok(find('[data-test-scale="increment"]:disabled'));
    assert.ok(find('[data-test-scale="decrement"]:disabled'));
    assert.ok(
      find('[data-test-scale-controls]')
        .getAttribute('aria-label')
        .includes("You aren't allowed"),
    );
  });
});
