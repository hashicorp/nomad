import { get } from '@ember/object';
import $ from 'jquery';
import { click, findAll, find, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moment from 'moment';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const sum = (list, key) => list.reduce((sum, item) => sum + get(item, key), 0);

let job;
let deployments;
let sortedDeployments;

moduleForAcceptance('Acceptance | job deployments', {
  beforeEach() {
    server.create('node');
    job = server.create('job');
    deployments = server.schema.deployments.where({ jobId: job.id });
    sortedDeployments = deployments.sort((a, b) => {
      const aVersion = server.db.jobVersions.findBy({ jobId: a.jobId, version: a.versionNumber });
      const bVersion = server.db.jobVersions.findBy({ jobId: b.jobId, version: b.versionNumber });
      if (aVersion.submitTime < bVersion.submitTime) {
        return 1;
      } else if (aVersion.submitTime > bVersion.submitTime) {
        return -1;
      }
      return 0;
    });
  },
});

test('/jobs/:id/deployments should list all job deployments', function(assert) {
  visit(`/jobs/${job.id}/deployments`);
  andThen(() => {
    assert.ok(
      findAll('[data-test-deployment]').length,
      deployments.length,
      'Each deployment gets a row in the timeline'
    );
  });
});

test('each deployment mentions the deployment shortId, status, version, and time since it was submitted', function(
  assert
) {
  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const version = server.db.jobVersions.findBy({
      jobId: deployment.jobId,
      version: deployment.versionNumber,
    });
    const deploymentRow = $(find('[data-test-deployment]'));

    assert.ok(deploymentRow.text().includes(deployment.id.split('-')[0]), 'Short ID');
    assert.equal(
      deploymentRow.find('[data-test-deployment-status]').text(),
      deployment.status,
      'Status'
    );
    assert.ok(
      deploymentRow
        .find('[data-test-deployment-status]')
        .hasClass(classForStatus(deployment.status)),
      'Status Class'
    );
    assert.ok(
      deploymentRow
        .find('[data-test-deployment-version]')
        .text()
        .includes(deployment.versionNumber),
      'Version #'
    );
    assert.ok(
      deploymentRow
        .find('[data-test-deployment-submit-time]')
        .text()
        .includes(moment(version.submitTime / 1000000).fromNow()),
      'Submit time ago'
    );
  });
});

test('when the deployment is running and needs promotion, the deployment item says so', function(
  assert
) {
  // Ensure the deployment needs deployment
  const deployment = sortedDeployments.models[0];
  const taskGroupSummary = deployment.deploymentTaskGroupSummaryIds.map(id =>
    server.schema.deploymentTaskGroupSummaries.find(id)
  )[0];

  deployment.update('status', 'running');
  deployment.save();

  taskGroupSummary.update({
    desiredCanaries: 1,
    placedCanaries: 0,
    promoted: false,
  });

  taskGroupSummary.save();

  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deploymentRow = find('[data-test-deployment]');
    assert.ok(
      deploymentRow.querySelector('[data-test-promotion-required]'),
      'Requires Promotion badge found'
    );
  });
});

test('each deployment item can be opened to show details', function(assert) {
  let deploymentRow;

  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    deploymentRow = find('[data-test-deployment]');

    assert.notOk(
      deploymentRow.querySelector('[data-test-deployment-details]'),
      'No deployment body'
    );

    click(deploymentRow.querySelector('[data-test-deployment-toggle-details]'));

    andThen(() => {
      assert.ok(
        deploymentRow.querySelector('[data-test-deployment-details]'),
        'Deployment body found'
      );
    });
  });
});

test('when open, a deployment shows the deployment metrics', function(assert) {
  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = find('[data-test-deployment]');
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    click(deploymentRow.querySelector('[data-test-deployment-toggle-details]'));

    andThen(() => {
      assert.equal(
        find('[data-test-deployment-metric="canaries"]').textContent.trim(),
        `${sum(taskGroupSummaries, 'placedCanaries')} / ${sum(
          taskGroupSummaries,
          'desiredCanaries'
        )}`,
        'Canaries, both places and desired, are in the metrics'
      );

      assert.equal(
        find('[data-test-deployment-metric="placed"]').textContent.trim(),
        sum(taskGroupSummaries, 'placedAllocs'),
        'Placed allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="desired"]').textContent.trim(),
        sum(taskGroupSummaries, 'desiredTotal'),
        'Desired allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="healthy"]').textContent.trim(),
        sum(taskGroupSummaries, 'healthyAllocs'),
        'Healthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-metric="unhealthy"]').textContent.trim(),
        sum(taskGroupSummaries, 'unhealthyAllocs'),
        'Unhealthy allocs aggregates across task groups'
      );

      assert.equal(
        find('[data-test-deployment-notification]').textContent.trim(),
        deployment.statusDescription,
        'Status description is in the metrics block'
      );
    });
  });
});

test('when open, a deployment shows a list of all task groups and their respective stats', function(
  assert
) {
  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = find('[data-test-deployment]');
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    click(deploymentRow.querySelector('[data-test-deployment-toggle-details]'));

    andThen(() => {
      const taskGroupTable = deploymentRow.querySelector('[data-test-deployment-task-groups]');

      assert.ok(taskGroupTable, 'Task groups found');

      assert.equal(
        taskGroupTable.querySelectorAll('[data-test-deployment-task-group]').length,
        taskGroupSummaries.length,
        'One row per task group'
      );

      const taskGroup = taskGroupSummaries[0];
      const taskGroupRow = taskGroupTable.querySelector('[data-test-deployment-task-group]');

      assert.equal(
        taskGroupRow.querySelector('[data-test-deployment-task-group-name]').textContent.trim(),
        taskGroup.name,
        'Name'
      );
      assert.equal(
        taskGroupRow
          .querySelector('[data-test-deployment-task-group-promotion]')
          .textContent.trim(),
        promotionTestForTaskGroup(taskGroup),
        'Needs Promotion'
      );
      assert.equal(
        taskGroupRow
          .querySelector('[data-test-deployment-task-group-auto-revert]')
          .textContent.trim(),
        taskGroup.autoRevert ? 'Yes' : 'No',
        'Auto Revert'
      );
      assert.equal(
        taskGroupRow.querySelector('[data-test-deployment-task-group-canaries]').textContent.trim(),
        `${taskGroup.placedCanaries} / ${taskGroup.desiredCanaries}`,
        'Canaries'
      );
      assert.equal(
        taskGroupRow.querySelector('[data-test-deployment-task-group-allocs]').textContent.trim(),
        `${taskGroup.placedAllocs} / ${taskGroup.desiredTotal}`,
        'Allocs'
      );
      assert.equal(
        taskGroupRow.querySelector('[data-test-deployment-task-group-healthy]').textContent.trim(),
        taskGroup.healthyAllocs,
        'Healthy Allocs'
      );
      assert.equal(
        taskGroupRow
          .querySelector('[data-test-deployment-task-group-unhealthy]')
          .textContent.trim(),
        taskGroup.unhealthyAllocs,
        'Unhealthy Allocs'
      );
    });
  });
});

test('when open, a deployment shows a list of all allocations for the deployment', function(
  assert
) {
  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = find('[data-test-deployment]');

    // TODO: Make this less brittle. This logic is copied from the mirage config,
    // since there is no reference to allocations on the deployment model.
    const allocations = server.db.allocations.where({ jobId: deployment.jobId }).slice(0, 3);

    click(deploymentRow.querySelector('[data-test-deployment-toggle-details]'));

    andThen(() => {
      assert.ok(
        deploymentRow.querySelector('[data-test-deployment-allocations]'),
        'Allocations found'
      );

      assert.equal(
        deploymentRow.querySelectorAll('[data-test-deployment-allocation]').length,
        allocations.length,
        'One row per allocation'
      );

      const allocation = allocations[0];
      const allocationRow = deploymentRow.querySelector('[data-test-deployment-allocation]');

      assert.equal(
        allocationRow.querySelector('[data-test-short-id]').textContent.trim(),
        allocation.id.split('-')[0],
        'Allocation is as expected'
      );
    });
  });
});

function classForStatus(status) {
  const classMap = {
    running: 'is-running',
    successful: 'is-primary',
    paused: 'is-light',
    failed: 'is-error',
    cancelled: 'is-cancelled',
  };

  return classMap[status] || 'is-dark';
}

function promotionTestForTaskGroup(taskGroup) {
  if (taskGroup.desiredCanaries > 0 && taskGroup.promoted === false) {
    return 'Yes';
  } else if (taskGroup.desiredCanaries > 0) {
    return 'No';
  }
  return 'N/A';
}
