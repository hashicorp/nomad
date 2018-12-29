import { click, findAll, find, visit } from 'ember-native-dom-helpers';
import Ember from 'ember';
import { test } from 'qunit';
import moment from 'moment';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

const { get, $ } = Ember;
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
      findAll('.timeline-object').length,
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
    const deploymentRow = $(findAll('.timeline-object')[0]);

    assert.ok(deploymentRow.text().includes(deployment.id.split('-')[0]), 'Short ID');
    assert.equal(deploymentRow.find('.tag').text(), deployment.status, 'Status');
    assert.ok(
      deploymentRow.find('.tag').hasClass(classForStatus(deployment.status)),
      'Status Class'
    );
    assert.ok(deploymentRow.text().includes(deployment.versionNumber), 'Version #');
    assert.ok(
      deploymentRow.text().includes(moment(version.submitTime / 1000000).fromNow()),
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
    const deploymentRow = $(findAll('.timeline-object')[0]);
    assert.ok(
      deploymentRow.find('.badge:contains("Requires Promotion")').length,
      'Requires Promotion badge found'
    );
  });
});

test('each deployment item can be opened to show details', function(assert) {
  let deploymentRow;

  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    deploymentRow = $(findAll('.timeline-object')[0]);

    assert.ok(deploymentRow.find('.boxed-section-body').length === 0, 'No deployment body');

    click(deploymentRow.find('button').get(0));

    andThen(() => {
      assert.ok(deploymentRow.find('.boxed-section-body').length, 'Deployment body found');
    });
  });
});

test('when open, a deployment shows the deployment metrics', function(assert) {
  visit(`/jobs/${job.id}/deployments`);

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = $(findAll('.timeline-object')[0]);
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    click(deploymentRow.find('button').get(0));

    andThen(() => {
      assert.equal(
        $('.deployment-metrics .label:contains("Canaries") + .value')
          .get(0)
          .textContent.trim(),
        `${sum(taskGroupSummaries, 'placedCanaries')} / ${sum(
          taskGroupSummaries,
          'desiredCanaries'
        )}`,
        'Canaries, both places and desired, are in the metrics'
      );

      assert.equal(
        $('.deployment-metrics .label:contains("Placed") + .value')
          .get(0)
          .textContent.trim(),
        sum(taskGroupSummaries, 'placedAllocs'),
        'Placed allocs aggregates across task groups'
      );

      assert.equal(
        $('.deployment-metrics .label:contains("Desired") + .value')
          .get(0)
          .textContent.trim(),
        sum(taskGroupSummaries, 'desiredTotal'),
        'Desired allocs aggregates across task groups'
      );

      assert.equal(
        $('.deployment-metrics .label:contains("Healthy") + .value')
          .get(0)
          .textContent.trim(),
        sum(taskGroupSummaries, 'healthyAllocs'),
        'Healthy allocs aggregates across task groups'
      );

      assert.equal(
        $('.deployment-metrics .label:contains("Unhealthy") + .value')
          .get(0)
          .textContent.trim(),
        sum(taskGroupSummaries, 'unhealthyAllocs'),
        'Unhealthy allocs aggregates across task groups'
      );

      assert.equal(
        find('.deployment-metrics .notification').textContent.trim(),
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
    const deploymentRow = $(findAll('.timeline-object')[0]);
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    click(deploymentRow.find('button').get(0));

    andThen(() => {
      assert.ok(
        deploymentRow.find('.boxed-section-head:contains("Task Groups")').length,
        'Task groups found'
      );

      const taskGroupTable = deploymentRow.find(
        '.boxed-section-head:contains("Task Groups") + .boxed-section-body tbody'
      );

      assert.equal(
        taskGroupTable.find('tr').length,
        taskGroupSummaries.length,
        'One row per task group'
      );

      const taskGroup = taskGroupSummaries[0];
      const taskGroupRow = taskGroupTable.find('tr:eq(0)');

      assert.equal(
        taskGroupRow
          .find('td:eq(0)')
          .text()
          .trim(),
        taskGroup.name,
        'Name'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(1)')
          .text()
          .trim(),
        promotionTestForTaskGroup(taskGroup),
        'Needs Promotion'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(2)')
          .text()
          .trim(),
        taskGroup.autoRevert ? 'Yes' : 'No',
        'Auto Revert'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(3)')
          .text()
          .trim(),
        `${taskGroup.placedCanaries} / ${taskGroup.desiredCanaries}`,
        'Canaries'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(4)')
          .text()
          .trim(),
        `${taskGroup.placedAllocs} / ${taskGroup.desiredTotal}`,
        'Allocs'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(5)')
          .text()
          .trim(),
        taskGroup.healthyAllocs,
        'Healthy Allocs'
      );
      assert.equal(
        taskGroupRow
          .find('td:eq(6)')
          .text()
          .trim(),
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
    const deploymentRow = $(findAll('.timeline-object')[0]);

    // TODO: Make this less brittle. This logic is copied from the mirage config,
    // since there is no reference to allocations on the deployment model.
    const allocations = server.db.allocations.where({ jobId: deployment.jobId }).slice(0, 3);

    click(deploymentRow.find('button').get(0));

    andThen(() => {
      assert.ok(
        deploymentRow.find('.boxed-section-head:contains("Allocations")').length,
        'Allocations found'
      );

      const allocationsTable = deploymentRow.find(
        '.boxed-section-head:contains("Allocations") + .boxed-section-body tbody'
      );

      assert.equal(
        allocationsTable.find('tr').length,
        allocations.length,
        'One row per allocation'
      );

      const allocation = allocations[0];
      const allocationRow = allocationsTable.find('tr:eq(0)');

      assert.equal(
        allocationRow
          .find('td:eq(0)')
          .text()
          .trim(),
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
