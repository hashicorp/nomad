import { get } from '@ember/object';
import { test } from 'qunit';
import moment from 'moment';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import Deployments from 'nomad-ui/tests/pages/jobs/job/deployments';

const sum = (list, key, getter = a => a) =>
  list.reduce((sum, item) => sum + getter(get(item, key)), 0);

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
  Deployments.visit({ id: job.id });

  andThen(() => {
    assert.ok(
      Deployments.deployments.length,
      deployments.length,
      'Each deployment gets a row in the timeline'
    );
  });
});

test('each deployment mentions the deployment shortId, status, version, and time since it was submitted', function(assert) {
  Deployments.visit({ id: job.id });

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const version = server.db.jobVersions.findBy({
      jobId: deployment.jobId,
      version: deployment.versionNumber,
    });
    const deploymentRow = Deployments.deployments.objectAt(0);

    assert.ok(deploymentRow.text.includes(deployment.id.split('-')[0]), 'Short ID');
    assert.equal(deploymentRow.status, deployment.status, 'Status');
    assert.ok(
      deploymentRow.statusClass.includes(classForStatus(deployment.status)),
      'Status Class'
    );
    assert.ok(deploymentRow.version.includes(deployment.versionNumber), 'Version #');
    assert.ok(
      deploymentRow.submitTime.includes(moment(version.submitTime / 1000000).fromNow()),
      'Submit time ago'
    );
  });
});

test('when the deployment is running and needs promotion, the deployment item says so', function(assert) {
  // Ensure the deployment needs deployment
  const deployment = sortedDeployments.models[0];
  const taskGroupSummary = deployment.deploymentTaskGroupSummaryIds.map(id =>
    server.schema.deploymentTaskGroupSummaries.find(id)
  )[0];

  deployment.update('status', 'running');
  deployment.save();

  taskGroupSummary.update({
    desiredCanaries: 1,
    placedCanaries: [],
    promoted: false,
  });

  taskGroupSummary.save();

  Deployments.visit({ id: job.id });

  andThen(() => {
    const deploymentRow = Deployments.deployments.objectAt(0);
    assert.ok(deploymentRow.promotionIsRequired, 'Requires Promotion badge found');
  });
});

test('each deployment item can be opened to show details', function(assert) {
  Deployments.visit({ id: job.id });

  andThen(() => {
    const deploymentRow = Deployments.deployments.objectAt(0);

    assert.notOk(deploymentRow.hasDetails, 'No deployment body');

    deploymentRow.toggle();

    andThen(() => {
      assert.ok(deploymentRow.hasDetails, 'Deployment body found');
    });
  });
});

test('when open, a deployment shows the deployment metrics', function(assert) {
  Deployments.visit({ id: job.id });

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = Deployments.deployments.objectAt(0);
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    deploymentRow.toggle();

    andThen(() => {
      assert.equal(
        deploymentRow.metricFor('canaries').text,
        `${sum(taskGroupSummaries, 'placedCanaries', a => a.length)} / ${sum(
          taskGroupSummaries,
          'desiredCanaries'
        )}`,
        'Canaries, both places and desired, are in the metrics'
      );

      assert.equal(
        deploymentRow.metricFor('placed').text,
        sum(taskGroupSummaries, 'placedAllocs'),
        'Placed allocs aggregates across task groups'
      );

      assert.equal(
        deploymentRow.metricFor('desired').text,
        sum(taskGroupSummaries, 'desiredTotal'),
        'Desired allocs aggregates across task groups'
      );

      assert.equal(
        deploymentRow.metricFor('healthy').text,
        sum(taskGroupSummaries, 'healthyAllocs'),
        'Healthy allocs aggregates across task groups'
      );

      assert.equal(
        deploymentRow.metricFor('unhealthy').text,
        sum(taskGroupSummaries, 'unhealthyAllocs'),
        'Unhealthy allocs aggregates across task groups'
      );

      assert.equal(
        deploymentRow.notification,
        deployment.statusDescription,
        'Status description is in the metrics block'
      );
    });
  });
});

test('when open, a deployment shows a list of all task groups and their respective stats', function(assert) {
  Deployments.visit({ id: job.id });

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = Deployments.deployments.objectAt(0);
    const taskGroupSummaries = deployment.deploymentTaskGroupSummaryIds.map(id =>
      server.db.deploymentTaskGroupSummaries.find(id)
    );

    deploymentRow.toggle();

    andThen(() => {
      assert.ok(deploymentRow.hasTaskGroups, 'Task groups found');

      assert.equal(
        deploymentRow.taskGroups.length,
        taskGroupSummaries.length,
        'One row per task group'
      );

      const taskGroup = taskGroupSummaries[0];
      const taskGroupRow = deploymentRow.taskGroups.objectAt(0);

      assert.equal(taskGroupRow.name, taskGroup.name, 'Name');
      assert.equal(taskGroupRow.promotion, promotionTestForTaskGroup(taskGroup), 'Needs Promotion');
      assert.equal(taskGroupRow.autoRevert, taskGroup.autoRevert ? 'Yes' : 'No', 'Auto Revert');
      assert.equal(
        taskGroupRow.canaries,
        `${taskGroup.placedCanaries.length} / ${taskGroup.desiredCanaries}`,
        'Canaries'
      );
      assert.equal(
        taskGroupRow.allocs,
        `${taskGroup.placedAllocs} / ${taskGroup.desiredTotal}`,
        'Allocs'
      );
      assert.equal(taskGroupRow.healthy, taskGroup.healthyAllocs, 'Healthy Allocs');
      assert.equal(taskGroupRow.unhealthy, taskGroup.unhealthyAllocs, 'Unhealthy Allocs');
      assert.equal(
        taskGroupRow.progress,
        moment(taskGroup.requireProgressBy).format('MM/DD/YY HH:mm:ss'),
        'Progress By'
      );
    });
  });
});

test('when open, a deployment shows a list of all allocations for the deployment', function(assert) {
  Deployments.visit({ id: job.id });

  andThen(() => {
    const deployment = sortedDeployments.models[0];
    const deploymentRow = Deployments.deployments.objectAt(0);

    // TODO: Make this less brittle. This logic is copied from the mirage config,
    // since there is no reference to allocations on the deployment model.
    const allocations = server.db.allocations.where({ jobId: deployment.jobId }).slice(0, 3);

    deploymentRow.toggle();

    andThen(() => {
      assert.ok(deploymentRow.hasAllocations, 'Allocations found');

      assert.equal(deploymentRow.allocations.length, allocations.length, 'One row per allocation');

      const allocation = allocations[0];
      const allocationRow = deploymentRow.allocations.objectAt(0);

      assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'Allocation is as expected');
    });
  });
});

test('when the job for the deployments is not found, an error message is shown, but the URL persists', function(assert) {
  Deployments.visit({ id: 'not-a-real-job' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job/deployments', 'The URL persists');
    assert.ok(Deployments.error.isPresent, 'Error message is shown');
    assert.equal(Deployments.error.title, 'Not Found', 'Error message is for 404');
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
