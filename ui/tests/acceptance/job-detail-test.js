/* eslint-disable ember/no-test-module-for */
/* eslint-disable qunit/require-expect */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import moment from 'moment';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import moduleForJob, {
  moduleForJobWithClientStatus,
} from 'nomad-ui/tests/helpers/module-for-job';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';

moduleForJob('Acceptance | job detail (batch)', 'allocations', () =>
  server.create('job', { type: 'batch', shallow: true })
);

moduleForJob('Acceptance | job detail (system)', 'allocations', () =>
  server.create('job', { type: 'system', shallow: true })
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (system)',
  () =>
    server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'system',
      createAllocations: false,
    })
);

moduleForJob('Acceptance | job detail (sysbatch)', 'allocations', () =>
  server.create('job', { type: 'sysbatch', shallow: true })
);

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch)',
  () =>
    server.create('job', {
      status: 'running',
      datacenters: ['dc1'],
      type: 'sysbatch',
      createAllocations: false,
    })
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
    });
  }
);

moduleForJob('Acceptance | job detail (sysbatch child)', 'allocations', () => {
  const parent = server.create('job', 'periodicSysbatch', {
    childrenCount: 1,
    shallow: true,
    datacenters: ['dc1'],
  });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJobWithClientStatus(
  'Acceptance | job detail with client status (sysbatch child)',
  () => {
    const parent = server.create('job', 'periodicSysbatch', {
      childrenCount: 1,
      shallow: true,
      datacenters: ['dc1'],
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
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJob(
  'Acceptance | job detail (periodic)',
  'children',
  () => server.create('job', 'periodic', { shallow: true }),
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
    'display namespace in children table': async function (job, assert) {
      assert.ok(JobDetail.jobsHeader.hasNamespace);
      assert.equal(JobDetail.jobs[0].namespace, job.namespace);
    },
  }
);

moduleForJob(
  'Acceptance | job detail (parameterized)',
  'children',
  () => server.create('job', 'parameterized', { shallow: true }),
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
    'display namespace in children table': async function (job, assert) {
      assert.ok(JobDetail.jobsHeader.hasNamespace);
      assert.equal(JobDetail.jobs[0].namespace, job.namespace);
    },
  }
);

moduleForJob('Acceptance | job detail (periodic child)', 'allocations', () => {
  const parent = server.create('job', 'periodic', {
    childrenCount: 1,
    shallow: true,
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
    });
    return server.db.jobs.where({ parentId: parent.id })[0];
  }
);

moduleForJob(
  'Acceptance | job detail (service)',
  'allocations',
  () => server.create('job', { type: 'service' }),
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

module('Acceptance | job detail (with namespaces)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let job, managementToken, clientToken;

  hooks.beforeEach(function () {
    server.createList('namespace', 2);
    server.create('node');
    job = server.create('job', {
      type: 'service',
      status: 'running',
      namespaceId: server.db.namespaces[1].name,
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
});
