import { click, findAll, currentURL, find, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import moduleForJob from 'nomad-ui/tests/helpers/module-for-job';

moduleForJob('Acceptance | job detail (batch)', () => server.create('job', { type: 'batch' }));
moduleForJob('Acceptance | job detail (system)', () => server.create('job', { type: 'system' }));
moduleForJob('Acceptance | job detail (periodic)', () => server.create('job', 'periodic'));

moduleForJob('Acceptance | job detail (parameterized)', () =>
  server.create('job', 'parameterized')
);

moduleForJob('Acceptance | job detail (periodic child)', () => {
  const parent = server.create('job', 'periodic');
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob('Acceptance | job detail (parameterized child)', () => {
  const parent = server.create('job', 'parameterized');
  return server.db.jobs.where({ parentId: parent.id })[0];
});

moduleForJob('Acceptance | job detail (service)', () => server.create('job', { type: 'service' }), {
  'the subnav links to deployment': (job, assert) => {
    click(find('[data-test-tab="deployments"] a'));
    andThen(() => {
      assert.equal(currentURL(), `/jobs/${job.id}/deployments`);
    });
  },
});

let job;

test('when the job is not found, an error message is shown, but the URL persists', function(assert) {
  visit('/jobs/not-a-real-job');

  andThen(() => {
    assert.equal(
      server.pretender.handledRequests.findBy('status', 404).url,
      '/v1/job/not-a-real-job',
      'A request to the nonexistent job is made'
    );
    assert.equal(currentURL(), '/jobs/not-a-real-job', 'The URL persists');
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
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
  visit(`/jobs/${job.id}?namespace=${namespace.name}`);

  andThen(() => {
    assert.ok(
      find('[data-test-job-stat="namespace"]').textContent.includes(namespace.name),
      'Namespace included in stats'
    );
  });
});

test('when switching namespaces, the app redirects to /jobs with the new namespace', function(assert) {
  const namespace = server.db.namespaces.find(job.namespaceId);
  const otherNamespace = server.db.namespaces.toArray().find(ns => ns !== namespace).name;
  const label = otherNamespace === 'default' ? 'Default Namespace' : otherNamespace;

  visit(`/jobs/${job.id}?namespace=${namespace.name}`);

  andThen(() => {
    selectChoose('[data-test-namespace-switcher]', label);
  });

  andThen(() => {
    assert.equal(currentURL().split('?')[0], '/jobs', 'Navigated to /jobs');
    const jobs = server.db.jobs
      .where({ namespace: otherNamespace })
      .sortBy('modifyIndex')
      .reverse();
    assert.equal(
      findAll('[data-test-job-row]').length,
      jobs.length,
      'Shows the right number of jobs'
    );
    jobs.forEach((job, index) => {
      const jobRow = findAll('[data-test-job-row]')[index];
      assert.equal(
        jobRow.querySelector('[data-test-job-name]').textContent.trim(),
        job.name,
        `Job ${index} is right`
      );
    });
  });
});
