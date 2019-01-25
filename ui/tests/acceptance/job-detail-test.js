import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moduleForJob from 'nomad-ui/tests/helpers/module-for-job';
import JobDetail from 'nomad-ui/tests/pages/jobs/detail';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

moduleForJob('Acceptance | job detail (batch)', 'allocations', () =>
  server.create('job', { type: 'batch' })
);
moduleForJob('Acceptance | job detail (system)', 'allocations', () =>
  server.create('job', { type: 'system' })
);
moduleForJob('Acceptance | job detail (periodic)', 'children', () =>
  server.create('job', 'periodic')
);

moduleForJob('Acceptance | job detail (parameterized)', 'children', () =>
  server.create('job', 'parameterized')
);

moduleForJob('Acceptance | job detail (periodic child)', 'allocations', () => {
  const parent = server.create('job', 'periodic');
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob('Acceptance | job detail (parameterized child)', 'allocations', () => {
  const parent = server.create('job', 'parameterized');
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob(
  'Acceptance | job detail (service)',
  'allocations',
  () => server.create('job', { type: 'service' }),
  {
    'the subnav links to deployment': (job, assert) => {
      JobDetail.tabFor('deployments').visit();
      andThen(() => {
        assert.equal(currentURL(), `/jobs/${job.id}/deployments`);
      });
    },
  }
);

let job;

test('when the job is not found, an error message is shown, but the URL persists', function(assert) {
  JobDetail.visit({ id: 'not-a-real-job' });

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
    assert.ok(JobDetail.error.isPresent, 'Error message is shown');
    assert.equal(JobDetail.error.title, 'Not Found', 'Error message is for 404');
  });
});

moduleForAcceptance('Acceptance | job detail (with namespaces)', {
  beforeEach() {
    server.createList('namespace', 2);
    server.create('node');
    job = server.create('job', { type: 'service', namespaceId: server.db.namespaces[1].name });
    server.createList('job', 3, { namespaceId: server.db.namespaces[0].name });
  },
});

test('when there are namespaces, the job detail page states the namespace for the job', function(assert) {
  const namespace = server.db.namespaces.find(job.namespaceId);
  JobDetail.visit({ id: job.id, namespace: namespace.name });

  andThen(() => {
    assert.ok(JobDetail.statFor('namespace').text, 'Namespace included in stats');
  });
});

test('when switching namespaces, the app redirects to /jobs with the new namespace', function(assert) {
  const namespace = server.db.namespaces.find(job.namespaceId);
  const otherNamespace = server.db.namespaces.toArray().find(ns => ns !== namespace).name;
  const label = otherNamespace === 'default' ? 'Default Namespace' : otherNamespace;

  JobDetail.visit({ id: job.id, namespace: namespace.name });

  andThen(() => {
    // TODO: Migrate to Page Objects
    selectChoose('[data-test-namespace-switcher]', label);
  });

  andThen(() => {
    assert.equal(currentURL().split('?')[0], '/jobs', 'Navigated to /jobs');

    const jobs = server.db.jobs
      .where({ namespace: otherNamespace })
      .sortBy('modifyIndex')
      .reverse();

    assert.equal(JobsList.jobs.length, jobs.length, 'Shows the right number of jobs');
    JobsList.jobs.forEach((jobRow, index) => {
      assert.equal(jobRow.name, jobs[index].name, `Job ${index} is right`);
    });
  });
});
