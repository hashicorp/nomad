/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember/no-test-module-for */
/* eslint-disable qunit/require-expect */
import { currentURL, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import moment from 'moment';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import moduleForJob, {
  moduleForJobWithClientStatus,
} from 'nomad-ui/tests/helpers/module-for-job';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import percySnapshot from '@percy/ember';
import { createRestartableJobs } from 'nomad-ui/mirage/scenarios/default';

moduleForJob('Acceptance | job detail (batch)', 'allocations', () =>
  server.create('job', {
    type: 'batch',
    shallow: true,
    noActiveDeployment: true,
    createAllocations: true,
    allocStatusDistribution: {
      running: 1,
    },
    withPreviousStableVersion: true,
  })
);

moduleForJob('Acceptance | job detail (system)', 'allocations', () =>
  server.create('job', {
    type: 'system',
    shallow: true,
    noActiveDeployment: true,
    createAllocations: true,
    allocStatusDistribution: {
      running: 1,
    },
    withPreviousStableVersion: true,
  })
);

moduleForJob('Acceptance | job detail (sysbatch)', 'allocations', () =>
  server.create('job', {
    type: 'sysbatch',
    shallow: true,
    noActiveDeployment: true,
    createAllocations: true,
    allocStatusDistribution: {
      running: 1,
      failed: 1,
    },
    withPreviousStableVersion: true,
  })
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch)',
  () => {
    server.create('namespace', { id: 'test' });
    return server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      createAllocations: false,
      noActiveDeployment: true,
      withPreviousStableVersion: true,
    });
  }
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch with namespace)',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    return server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      namespaceId: namespace.name,
      createAllocations: false,
      noActiveDeployment: true,
      withPreviousStableVersion: true,
    });
  }
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch with namespace and wildcard dc)',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    return server.create('job', {
      status: 'running',
      datacenters: ['*'],
      type: 'sysbatch',
      namespaceId: namespace.name,
      createAllocations: false,
      noActiveDeployment: true,
      withPreviousStableVersion: true,
    });
  }
);

moduleForJob('Acceptance | job detail (sysbatch child)', 'allocations', () => {
  const parent = server.create('job', 'periodicSysbatch', {
    childrenCount: 1,
    shallow: true,
    datacenters: ['dc1'],
    createAllocations: true,
    allocStatusDistribution: {
      running: 1,
    },
    noActiveDeployment: true,
    withPreviousStableVersion: true,
  });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch child)',
  () => {
    server.create('namespace', { id: 'test' });
    const parent = server.create('job', 'periodicSysbatch', {
      childrenCount: 1,
      shallow: true,
      datacenters: ['dc1'],
      noActiveDeployment: true,
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch child with namespace)',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    const parent = server.create('job', 'periodicSysbatch', {
      childrenCount: 1,
      shallow: true,
      namespaceId: namespace.name,
      datacenters: ['dc1'],
      noActiveDeployment: true,
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch child with namespace and wildcard dc)',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    const parent = server.create('job', 'periodicSysbatch', {
      childrenCount: 1,
      shallow: true,
      namespaceId: namespace.name,
      datacenters: ['*'],
      noActiveDeployment: true,
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJob(
  'Acceptance | job detail (periodic)',
  'children',
  () =>
    server.create('job', 'periodic', {
      shallow: true,
      withPreviousStableVersion: true,
    }),
  {
    'the default sort is submitTime descending': async function (job, assert) {
      const mostRecentLaunch = server.db.jobs
        .where({ parentId: job.id })
        .sortBy('submitTime')
        .reverse()[0];

      assert.ok(JobDetail.jobsHeader.hasSubmitTime);
      assert.equal(
        JobDetail.jobs[0].submitTime,
        moment(mostRecentLaunch.submitTime / 1000000).format(
          'MMM DD HH:mm:ss ZZ'
        )
      );
    },
    "don't display redundant information in children table": async function (
      job,
      assert
    ) {
      assert.notOk(JobDetail.jobsHeader.hasNodePool);
      assert.notOk(JobDetail.jobsHeader.hasPriority);
      assert.notOk(JobDetail.jobsHeader.hasType);
    },
  }
);

moduleForJob(
  'Acceptance | job detail (periodic in namespace)',
  'children',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    const parent = server.create('job', 'periodic', {
      shallow: true,
      namespaceId: namespace.name,
    });
    return parent;
  },
  {
    "don't display namespace in children table": async function (job, assert) {
      assert.notOk(JobDetail.jobsHeader.hasNamespace);
    },
  }
);

moduleForJob(
  'Acceptance | job detail (parameterized)',
  'children',
  () =>
    server.create('job', 'parameterized', {
      shallow: true,
      noActiveDeployment: true,
      withPreviousStableVersion: true,
    }),
  {
    'the default sort is submitTime descending': async (job, assert) => {
      const mostRecentLaunch = server.db.jobs
        .where({ parentId: job.id })
        .sortBy('submitTime')
        .reverse()[0];

      assert.ok(JobDetail.jobsHeader.hasSubmitTime);
      assert.equal(
        JobDetail.jobs[0].submitTime,
        moment(mostRecentLaunch.submitTime / 1000000).format(
          'MMM DD HH:mm:ss ZZ'
        )
      );
    },
    "don't display redundant information in children table": async function (
      job,
      assert
    ) {
      assert.notOk(JobDetail.jobsHeader.hasNodePool);
      assert.notOk(JobDetail.jobsHeader.hasPriority);
      assert.notOk(JobDetail.jobsHeader.hasType);
    },
  }
);

moduleForJob(
  'Acceptance | job detail (parameterized in namespace)',
  'children',
  () => {
    const namespace = server.create('namespace', { id: 'test' });
    const parent = server.create('job', 'parameterized', {
      shallow: true,
      namespaceId: namespace.name,
    });
    return parent;
  },
  {
    "don't display namespace in children table": async function (job, assert) {
      assert.notOk(JobDetail.jobsHeader.hasNamespace);
    },
  }
);

moduleForJob('Acceptance | job detail (periodic child)', 'allocations', () => {
  const parent = server.create('job', 'periodic', {
    childrenCount: 1,
    shallow: true,
    createAllocations: true,
    allocStatusDistribution: {
      running: 1,
    },
    noActiveDeployment: true,
    withPreviousStableVersion: true,
  });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob(
  'Acceptance | job detail (parameterized child)',
  'allocations',
  () => {
    const parent = server.create('job', 'parameterized', {
      childrenCount: 1,
      shallow: true,
      noActiveDeployment: true,
      createAllocations: true,
      allocStatusDistribution: {
        running: 1,
      },
      // Child's gotta be non-queued to be able to run
      status: 'running', //  TODO: TEMP
      withPreviousStableVersion: true,
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJob(
  'Acceptance | job detail (service)',
  'allocations',
  () =>
    server.create('job', {
      type: 'service',
      noActiveDeployment: true,
      withPreviousStableVersion: true,
      allocStatusDistribution: {
        running: 1,
      },
    }),
  {
    'the subnav links to deployment': async (job, assert) => {
      await JobDetail.tabFor('deployments').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/deployments`);
    },
    'when the job is not found, an error message is shown, but the URL persists':
      async (job, assert) => {
        await JobDetail.visit({ id: 'not-a-real-job' });

        assert.equal(
          server.pretender.handledRequests
            .filter((request) => !request.url.includes('policy'))
            .findBy('status', 404).url,
          '/v1/job/not-a-real-job',
          'A request to the nonexistent job is made'
        );
        assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
        assert.ok(JobDetail.error.isPresent, 'Error message is shown');
        assert.equal(
          JobDetail.error.title,
          'Not Found',
          'Error message is for 404'
        );
      },
  }
);

module('Acceptance | ui block', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function () {
    window.localStorage.clear();
    server.create('agent');
    server.create('node-pool');
    server.create('node');

    server.create('job', {
      name: 'hcl-definition-job',
      id: 'display-hcl',
      namespaceId: 'default',
    });

    server.create('job', {
      name: 'ui-block-job',
      id: 'ui-block-job',
      ui: {
        Links: [
          {
            Label: 'HashiCorp',
            Url: 'https://hashicorp.com',
          },
          {
            Label: 'Nomad',
            Url: 'https://nomadproject.io',
          },
        ],
        Description:
          'A job with a UI-block defined description and links. It has **bold text** and everything!',
      },
    });
  });

  test('job renders with description', async function (assert) {
    window.localStorage.clear();
    await JobDetail.visit({ id: 'hcl-definition-job' });
    assert
      .dom('[data-test-job-description]')
      .doesNotExist('Job description does not exist on a standard job');
    await JobDetail.visit({ id: 'ui-block-job' });
    assert
      .dom('[data-test-job-description]')
      .exists('Job description exists when defined in HCL');
    assert
      .dom('[data-test-job-description] strong')
      .exists('Job description is rendered as markdown, with bold text');
  });

  test('job renders with links', async function (assert) {
    window.localStorage.clear();
    await JobDetail.visit({ id: 'hcl-definition-job' });
    assert
      .dom('[data-test-job-links]')
      .doesNotExist('Job links do not exist on a standard job');
    await JobDetail.visit({ id: 'ui-block-job' });
    assert
      .dom('[data-test-job-links] a')
      .exists({ count: 2 }, 'Job links exists when defined in HCL');
    await percySnapshot(assert, {
      percyCSS: `
        .allocation-row td { display: none; }
      `,
    });
  });

  test('job sanitizes input', async function (assert) {
    server.create('node-pool');
    server.create('node');
    server.create('job', {
      id: 'xss-job',
      ui: {
        Description: '<script>alert("XSS");</script><p>Safe text</p>',
      },
    });

    await JobDetail.visit({ id: 'xss-job' });

    assert
      .dom('[data-test-job-description]')
      .hasText('Safe text', 'Description should only contain safe text');

    assert
      .dom('[data-test-job-description] script')
      .doesNotExist('Should not render script tags');
  });
});

module('Acceptance | job detail (with namespaces)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let job, managementToken, clientToken;

  hooks.beforeEach(function () {
    server.createList('namespace', 2);
    server.create('node-pool');
    server.create('node');
    job = server.create('job', {
      type: 'service',
      status: 'running',
      namespaceId: server.db.namespaces[1].name,
      noActiveDeployment: true,
    });
    server.createList('job', 3, {
      namespaceId: server.db.namespaces[0].name,
    });

    managementToken = server.create('token');
    clientToken = server.create('token');
  });

  test('it passes an accessibility audit', async function (assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: `${job.id}@${namespace.name}` });
    await a11yAudit(assert);
  });

  test('when there are namespaces, the job detail page states the namespace for the job', async function (assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);

    await JobDetail.visit({
      id: `${job.id}@${namespace.name}`,
    });

    assert.ok(
      JobDetail.statFor('namespace').text,
      'Namespace included in stats'
    );
  });

  test('the exec button state can change between namespaces', async function (assert) {
    const job1 = server.create('job', {
      status: 'running',
      namespaceId: server.db.namespaces[0].id,
    });
    const job2 = server.create('job', {
      status: 'running',
      namespaceId: server.db.namespaces[1].id,
    });

    window.localStorage.nomadTokenSecret = clientToken.secretId;

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: job1.namespaceId,
            Capabilities: ['list-jobs', 'alloc-exec'],
          },
          {
            Name: job2.namespaceId,
            Capabilities: ['list-jobs'],
          },
        ],
      },
    });

    clientToken.policyIds = [policy.id];
    clientToken.save();

    await JobDetail.visit({ id: job1.id });
    assert.notOk(JobDetail.execButton.isDisabled);

    const secondNamespace = server.db.namespaces[1];
    await JobDetail.visit({ id: `${job2.id}@${secondNamespace.name}` });

    assert.ok(JobDetail.execButton.isDisabled);
  });

  test('the anonymous policy is fetched to check whether to show the exec button', async function (assert) {
    window.localStorage.removeItem('nomadTokenSecret');

    server.create('policy', {
      id: 'anonymous',
      name: 'anonymous',
      rulesJSON: {
        Namespaces: [
          {
            Name: 'default',
            Capabilities: ['list-jobs', 'alloc-exec'],
          },
        ],
      },
    });

    await JobDetail.visit({
      id: `${job.id}@${server.db.namespaces[1].name}`,
    });

    assert.notOk(JobDetail.execButton.isDisabled);
  });

  test('meta table is displayed if job has meta attributes', async function (assert) {
    const jobWithMeta = server.create('job', {
      status: 'running',
      namespaceId: server.db.namespaces[1].id,
      meta: {
        'a.b': 'c',
      },
    });

    await JobDetail.visit({
      id: `${job.id}@${server.db.namespaces[1].name}`,
    });

    assert.notOk(JobDetail.metaTable, 'Meta table not present');

    await JobDetail.visit({
      id: `${jobWithMeta.id}@${server.db.namespaces[1].name}`,
    });
    assert.ok(JobDetail.metaTable, 'Meta table is present');
  });

  test('pack details are displayed', async function (assert) {
    const namespace = server.db.namespaces[1].id;
    const jobFromPack = server.create('job', {
      status: 'running',
      namespaceId: namespace,
      meta: {
        'pack.name': 'my-pack',
        'pack.version': '1.0.0',
      },
    });

    await JobDetail.visit({ id: `${jobFromPack.id}@${namespace}` });

    assert.ok(JobDetail.packTag, 'Pack tag is present');
    assert.equal(
      JobDetail.packStatFor('name').text,
      `Name ${jobFromPack.meta['pack.name']}`,
      `Pack name is ${jobFromPack.meta['pack.name']}`
    );
    assert.equal(
      JobDetail.packStatFor('version').text,
      `Version ${jobFromPack.meta['pack.version']}`,
      `Pack version is ${jobFromPack.meta['pack.version']}`
    );
  });

  test('resource recommendations show when they exist and can be expanded, collapsed, and processed', async function (assert) {
    server.create('feature', { name: 'Dynamic Application Sizing' });

    job = server.create('job', {
      type: 'service',
      status: 'running',
      namespaceId: server.db.namespaces[1].name,
      groupsCount: 3,
      createRecommendations: true,
      noActiveDeployment: true,
    });

    window.localStorage.nomadTokenSecret = managementToken.secretId;
    await JobDetail.visit({
      id: `${job.id}@${server.db.namespaces[1].name}`,
    });

    const groupsWithRecommendations = job.taskGroups.filter((group) =>
      group.tasks.models.any((task) => task.recommendations.models.length)
    );
    const jobRecommendationCount = groupsWithRecommendations.length;

    const firstRecommendationGroup = groupsWithRecommendations.models[0];

    assert.equal(JobDetail.recommendations.length, jobRecommendationCount);

    const recommendation = JobDetail.recommendations[0];

    assert.equal(recommendation.group, firstRecommendationGroup.name);
    assert.ok(recommendation.card.isHidden);

    const toggle = recommendation.toggleButton;

    assert.equal(toggle.text, 'Show');

    await toggle.click();

    assert.ok(recommendation.card.isPresent);
    assert.equal(toggle.text, 'Collapse');

    await toggle.click();

    assert.ok(recommendation.card.isHidden);

    await toggle.click();

    assert.equal(
      recommendation.card.slug.groupName,
      firstRecommendationGroup.name
    );

    await recommendation.card.acceptButton.click();

    assert.equal(JobDetail.recommendations.length, jobRecommendationCount - 1);

    await JobDetail.tabFor('definition').visit();
    await JobDetail.tabFor('overview').visit();

    assert.equal(JobDetail.recommendations.length, jobRecommendationCount - 1);
  });

  test('resource recommendations are not fetched when the feature doesn’t exist', async function (assert) {
    window.localStorage.nomadTokenSecret = managementToken.secretId;
    await JobDetail.visit({
      id: `${job.id}@${server.db.namespaces[1].name}`,
    });

    assert.equal(JobDetail.recommendations.length, 0);

    assert.equal(
      server.pretender.handledRequests.filter((request) =>
        request.url.includes('recommendations')
      ).length,
      0
    );
  });

  test('when the dynamic autoscaler is applied, you can scale a task within the job detail page', async function (assert) {
    const SCALE_AND_WRITE_NAMESPACE = 'scale-and-write-namespace';
    const READ_ONLY_NAMESPACE = 'read-only-namespace';
    const clientToken = server.create('token');

    const namespace = server.create('namespace', {
      id: SCALE_AND_WRITE_NAMESPACE,
    });
    const secondNamespace = server.create('namespace', {
      id: READ_ONLY_NAMESPACE,
    });

    job = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: SCALE_AND_WRITE_NAMESPACE,
    });

    const job2 = server.create('job', {
      groupCount: 0,
      createAllocations: false,
      shallow: true,
      noActiveDeployment: true,
      namespaceId: READ_ONLY_NAMESPACE,
    });
    const scalingGroup2 = server.create('task-group', {
      job: job2,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job2.update({ taskGroupIds: [scalingGroup2.id] });

    const policy = server.create('policy', {
      id: 'something',
      name: 'something',
      rulesJSON: {
        Namespaces: [
          {
            Name: SCALE_AND_WRITE_NAMESPACE,
            Capabilities: ['scale-job', 'submit-job', 'read-job', 'list-jobs'],
          },
          {
            Name: READ_ONLY_NAMESPACE,
            Capabilities: ['list-jobs', 'read-job'],
          },
        ],
      },
    });
    const scalingGroup = server.create('task-group', {
      job,
      name: 'scaling',
      count: 1,
      shallow: true,
      withScaling: true,
    });
    job.update({ taskGroupIds: [scalingGroup.id] });

    clientToken.policyIds = [policy.id];
    clientToken.save();
    window.localStorage.nomadTokenSecret = clientToken.secretId;

    await JobDetail.visit({ id: `${job.id}@${namespace.name}` });
    assert.notOk(JobDetail.incrementButton.isDisabled);

    await JobDetail.visit({ id: `${job2.id}@${secondNamespace.name}` });
    assert.ok(JobDetail.incrementButton.isDisabled);
  });

  test('handles when a job is remotely purged', async function (assert) {
    const namespace = server.create('namespace');
    const job = server.create('job', {
      namespaceId: namespace.id,
      status: 'running',
      type: 'service',
      shallow: true,
      noActiveDeployment: true,
      createAllocations: true,
      groupsCount: 1,
      groupAllocCount: 1,
      allocStatusDistribution: {
        running: 1,
      },
    });

    await JobDetail.visit({ id: `${job.id}@${namespace.id}` });

    assert.equal(currentURL(), `/jobs/${job.id}%40${namespace.id}`);

    // Simulate a 404 error on the job watcher
    const controller = this.owner.lookup('controller:jobs.job');
    let jobWatcher = controller.watchers.job;
    jobWatcher.isError = true;
    jobWatcher.error = { errors: [{ status: '404' }] };
    await settled();

    // User should be booted off the page
    assert.equal(currentURL(), '/jobs');

    // A notification should be present
    assert
      .dom('.flash-message.alert-critical')
      .exists('A toast error message pops up.');

    await percySnapshot(assert);
  });

  test('handles when a job is remotely purged, from a job subnav page', async function (assert) {
    const namespace = server.create('namespace');
    const job = server.create('job', {
      namespaceId: namespace.id,
      status: 'running',
      type: 'service',
      shallow: true,
      noActiveDeployment: true,
      createAllocations: true,
      groupsCount: 1,
      groupAllocCount: 1,
      allocStatusDistribution: {
        running: 1,
      },
    });

    await JobDetail.visit({ id: `${job.id}@${namespace.id}` });
    await JobDetail.tabFor('allocations').visit();

    assert.equal(currentURL(), `/jobs/${job.id}@${namespace.id}/allocations`);

    // Simulate a 404 error on the job watcher
    const controller = this.owner.lookup('controller:jobs.job');
    let jobWatcher = controller.watchers.job;
    jobWatcher.isError = true;
    jobWatcher.error = { errors: [{ status: '404' }] };
    await settled();

    // User should be booted off the page
    assert.equal(currentURL(), '/jobs');

    // A notification should be present
    assert
      .dom('.flash-message.alert-critical')
      .exists('A toast error message pops up.');
  });
});

module('Job Start/Stop/Revert/Edit and Resubmit', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    server.create('agent');
    server.create('node-pool');
    server.create('node');

    createRestartableJobs(server);
  });

  test('Start Job depends on the job being stopped', async function (assert) {
    const restartableJob = server.db.jobs.findBy(
      (j) => j.name === 'restartable-job'
    );
    const revertableJob = server.db.jobs.findBy(
      (j) => j.name === 'revertable-job'
    );
    const nonRevertableJob = server.db.jobs.findBy(
      (j) => j.name === 'non-revertable-job'
    );
    await JobDetail.visit({ id: restartableJob.id });

    assert.ok(JobDetail.start.isPresent);
    assert.notOk(JobDetail.stop.isPresent);
    assert.notOk(JobDetail.revert.isPresent);
    assert.notOk(JobDetail.editAndResubmit.isPresent);
    await percySnapshot('Start Job depends on the job being stopped');

    await JobDetail.visit({ id: revertableJob.id });
    assert.notOk(JobDetail.start.isPresent);

    await percySnapshot('Revertable Job depends on having stable job versions');

    await JobDetail.visit({ id: nonRevertableJob.id });
    assert.notOk(JobDetail.start.isPresent);
    await percySnapshot(
      'Non-revertable Job depends on having no stable job versions'
    );
  });

  test('A revertable job depends on having stable job versions', async function (assert) {
    const revertableJob = server.db.jobs.findBy(
      (j) => j.name === 'revertable-job'
    );
    const nonRevertableJob = server.db.jobs.findBy(
      (j) => j.name === 'non-revertable-job'
    );
    await JobDetail.visit({ id: revertableJob.id });

    assert.ok(JobDetail.revert.isPresent);
    assert.equal(JobDetail.revert.text, 'Revert to last stable version (v1)');

    await JobDetail.visit({ id: nonRevertableJob.id });
    assert.notOk(JobDetail.revert.isPresent);
    assert.ok(JobDetail.editAndResubmit.isPresent);
  });

  test('A batch job with a previous version can be reverted', async function (assert) {
    const revertableSystemJob = server.db.jobs.findBy(
      (j) => j.name === 'revertable-batch-job'
    );
    await JobDetail.visit({ id: revertableSystemJob.id });
    assert.ok(JobDetail.revert.isPresent);
    assert.equal(JobDetail.revert.text, 'Revert to last version (v0)');
  });

  test('Clicking the resubmit button navigates to the job definition page in edit mode', async function (assert) {
    const job = server.db.jobs.findBy((j) => j.name === 'non-revertable-job');
    await JobDetail.visit({ id: job.id });
    await JobDetail.editAndResubmit.click();
    assert.equal(
      currentURL(),
      `/jobs/${job.id}/definition?isEditing=true&view=job-spec`
    );
  });
});
