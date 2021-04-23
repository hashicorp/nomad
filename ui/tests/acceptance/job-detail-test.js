/* eslint-disable ember/no-test-module-for */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import moment from 'moment';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import moduleForJob from 'nomad-ui/tests/helpers/module-for-job';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';

moduleForJob('Acceptance | job detail (batch)', 'allocations', () =>
  server.create('job', { type: 'batch', shallow: true })
);
moduleForJob('Acceptance | job detail (system)', 'allocations', () =>
  server.create('job', { type: 'system', shallow: true })
);
moduleForJob(
  'Acceptance | job detail (periodic)',
  'children',
  () => server.create('job', 'periodic', { shallow: true }),
  {
    'the default sort is submitTime descending': async function(job, assert) {
      const mostRecentLaunch = server.db.jobs
        .where({ parentId: job.id })
        .sortBy('submitTime')
        .reverse()[0];

      assert.equal(
        JobDetail.jobs[0].submitTime,
        moment(mostRecentLaunch.submitTime / 1000000).format('MMM DD HH:mm:ss ZZ')
      );
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

      assert.equal(
        JobDetail.jobs[0].submitTime,
        moment(mostRecentLaunch.submitTime / 1000000).format('MMM DD HH:mm:ss ZZ')
      );
    },
  }
);

moduleForJob('Acceptance | job detail (periodic child)', 'allocations', () => {
  const parent = server.create('job', 'periodic', { childrenCount: 1, shallow: true });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob('Acceptance | job detail (parameterized child)', 'allocations', () => {
  const parent = server.create('job', 'parameterized', { childrenCount: 1, shallow: true });
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob(
  'Acceptance | job detail (service)',
  'allocations',
  () => server.create('job', { type: 'service' }),
  {
    'the subnav links to deployment': async (job, assert) => {
      await JobDetail.tabFor('deployments').visit();
      assert.equal(currentURL(), `/jobs/${job.id}/deployments`);
    },
    'when the job is not found, an error message is shown, but the URL persists': async (
      job,
      assert
    ) => {
      await JobDetail.visit({ id: 'not-a-real-job' });

      assert.equal(
        server.pretender.handledRequests
          .filter(request => !request.url.includes('policy'))
          .findBy('status', 404).url,
        '/v1/job/not-a-real-job',
        'A request to the nonexistent job is made'
      );
      assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
      assert.ok(JobDetail.error.isPresent, 'Error message is shown');
      assert.equal(JobDetail.error.title, 'Not Found', 'Error message is for 404');
    },
  }
);

module('Acceptance | job detail (with namespaces)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let job, managementToken, clientToken;

  hooks.beforeEach(function() {
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

  test('it passes an accessibility audit', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });
    await a11yAudit(assert);
  });

  test('when there are namespaces, the job detail page states the namespace for the job', async function(assert) {
    const namespace = server.db.namespaces.find(job.namespaceId);
    await JobDetail.visit({ id: job.id, namespace: namespace.name });

    assert.ok(JobDetail.statFor('namespace').text, 'Namespace included in stats');
  });

  test('the exec button state can change between namespaces', async function(assert) {
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
    await JobDetail.visit({ id: job2.id, namespace: secondNamespace.name });
    assert.ok(JobDetail.execButton.isDisabled);
  });

  test('the anonymous policy is fetched to check whether to show the exec button', async function(assert) {
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

    await JobDetail.visit({ id: job.id, namespace: server.db.namespaces[1].name });
    assert.notOk(JobDetail.execButton.isDisabled);
  });

  test('resource recommendations show when they exist and can be expanded, collapsed, and processed', async function(assert) {
    server.create('feature', { name: 'Dynamic Application Sizing' });

    job = server.create('job', {
      type: 'service',
      status: 'running',
      namespaceId: server.db.namespaces[1].name,
      groupsCount: 3,
      createRecommendations: true,
    });

    window.localStorage.nomadTokenSecret = managementToken.secretId;
    await JobDetail.visit({ id: job.id, namespace: server.db.namespaces[1].name });

    const groupsWithRecommendations = job.taskGroups.filter(group =>
      group.tasks.models.any(task => task.recommendations.models.length)
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

    assert.equal(recommendation.card.slug.groupName, firstRecommendationGroup.name);

    await recommendation.card.acceptButton.click();

    assert.equal(JobDetail.recommendations.length, jobRecommendationCount - 1);

    await JobDetail.tabFor('definition').visit();
    await JobDetail.tabFor('overview').visit();

    assert.equal(JobDetail.recommendations.length, jobRecommendationCount - 1);
  });

  test('resource recommendations are not fetched when the feature doesn’t exist', async function(assert) {
    window.localStorage.nomadTokenSecret = managementToken.secretId;
    await JobDetail.visit({ id: job.id, namespace: server.db.namespaces[1].name });

    assert.equal(JobDetail.recommendations.length, 0);

    assert.equal(
      server.pretender.handledRequests.filter(request => request.url.includes('recommendations'))
        .length,
      0
    );
  });
});
