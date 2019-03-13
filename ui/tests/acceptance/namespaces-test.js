import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import JobsList from 'nomad-ui/tests/pages/jobs/list';

module('Acceptance | namespaces (disabled)', function(hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function() {
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  });

  test('the namespace switcher is not in the gutter menu', function(assert) {
    JobsList.visit();

    assert.notOk(JobsList.namespaceSwitcher.isPresent, 'No namespace switcher found');
  });

  test('the jobs request is made with no query params', function(assert) {
    JobsList.visit();

    const request = server.pretender.handledRequests.findBy('url', '/v1/jobs');
    assert.equal(request.queryParams.namespace, undefined, 'No namespace query param');
  });
});

module('Acceptance | namespaces (enabled)', function(hooks) {
  setupApplicationTest(hooks);

  hooks.beforeEach(function() {
    server.createList('namespace', 3);
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  });

  test('the namespace switcher lists all namespaces', function(assert) {
    const namespaces = server.db.namespaces;

    JobsList.visit();

    assert.ok(JobsList.namespaceSwitcher.isPresent, 'Namespace switcher found');
    JobsList.namespaceSwitcher.open();
    // TODO this selector should be scoped to only the namespace switcher options,
    // but ember-wormhole makes that difficult.
    assert.equal(
      JobsList.namespaceSwitcher.options.length,
      namespaces.length,
      'All namespaces are in the switcher'
    );
    assert.equal(
      JobsList.namespaceSwitcher.options.objectAt(0).label,
      'Default Namespace',
      'The first namespace is always the default one'
    );

    const sortedNamespaces = namespaces.slice(1).sortBy('name');
    JobsList.namespaceSwitcher.options.forEach((option, index) => {
      // Default Namespace handled separately
      if (index === 0) return;

      const namespace = sortedNamespaces[index - 1];
      assert.equal(option.label, namespace.name, `index ${index}: ${namespace.name}`);
    });
  });

  test('changing the namespace sets the namespace in localStorage', function(assert) {
    const namespace = server.db.namespaces[1];

    JobsList.visit();

    selectChoose('[data-test-namespace-switcher]', namespace.name);
    assert.equal(
      window.localStorage.nomadActiveNamespace,
      namespace.id,
      'Active namespace was set'
    );
  });

  test('changing the namespace refreshes the jobs list when on the jobs page', function(assert) {
    const namespace = server.db.namespaces[1];

    JobsList.visit();

    const requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 1, 'First request to jobs');
    assert.equal(
      requests[0].queryParams.namespace,
      undefined,
      'Namespace query param is defaulted to "default"/undefined'
    );

    // TODO: handle this with Page Objects
    selectChoose('[data-test-namespace-switcher]', namespace.name);

    const requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 2, 'Second request to jobs');
    assert.equal(
      requests[1].queryParams.namespace,
      namespace.name,
      'Namespace query param on second request'
    );
  });
});
