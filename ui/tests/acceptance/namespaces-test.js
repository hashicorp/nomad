import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { selectChoose } from 'ember-power-select/test-support';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import Allocation from 'nomad-ui/tests/pages/allocations/detail';
import PluginsList from 'nomad-ui/tests/pages/storage/plugins/list';
import VolumesList from 'nomad-ui/tests/pages/storage/volumes/list';

module('Acceptance | namespaces (disabled)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('agent');
    server.create('node');
    server.createList('job', 5, { createAllocations: false });
  });

  test('the namespace switcher is not in the gutter menu', async function(assert) {
    await JobsList.visit();
    assert.notOk(JobsList.namespaceSwitcher.isPresent, 'No namespace switcher found');
  });

  test('the jobs request is made with no query params', async function(assert) {
    await JobsList.visit();

    const request = server.pretender.handledRequests.findBy('url', '/v1/jobs');
    assert.equal(request.queryParams.namespace, undefined, 'No namespace query param');
  });
});

module('Acceptance | namespaces (enabled)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.createList('namespace', 3);
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  });

  hooks.afterEach(function() {
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function(assert) {
    await JobsList.visit();
    await a11yAudit(assert);
  });

  test('the namespace switcher lists all namespaces', async function(assert) {
    const namespaces = server.db.namespaces;

    await JobsList.visit();

    assert.ok(JobsList.namespaceSwitcher.isPresent, 'Namespace switcher found');
    await JobsList.namespaceSwitcher.open();
    // TODO this selector should be scoped to only the namespace switcher options,
    // but ember-wormhole makes that difficult.
    assert.equal(
      JobsList.namespaceSwitcher.options.length,
      namespaces.length,
      'All namespaces are in the switcher'
    );
    assert.equal(
      JobsList.namespaceSwitcher.options.objectAt(0).label,
      'Namespace: default',
      'The first namespace is always the default one'
    );

    const sortedNamespaces = namespaces.slice(1).sortBy('name');
    JobsList.namespaceSwitcher.options.forEach((option, index) => {
      // Default Namespace handled separately
      if (index === 0) return;

      const namespace = sortedNamespaces[index - 1];
      assert.equal(
        option.label,
        `Namespace: ${namespace.name}`,
        `index ${index}: ${namespace.name}`
      );
    });
  });

  test('changing the namespace sets the namespace in localStorage', async function(assert) {
    const namespace = server.db.namespaces[1];

    await JobsList.visit();
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    assert.equal(
      window.localStorage.nomadActiveNamespace,
      namespace.id,
      'Active namespace was set'
    );
  });

  test('changing the namespace refreshes the jobs list when on the jobs page', async function(assert) {
    const namespace = server.db.namespaces[1];

    await JobsList.visit();

    let requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 1, 'First request to jobs');
    assert.equal(
      requests[0].queryParams.namespace,
      undefined,
      'Namespace query param is defaulted to "default"/undefined'
    );

    // TODO: handle this with Page Objects
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 2, 'Second request to jobs');
    assert.equal(
      requests[1].queryParams.namespace,
      namespace.name,
      'Namespace query param on second request'
    );
  });

  test('changing the namespace in the clients hierarchy navigates to the jobs page', async function(assert) {
    const namespace = server.db.namespaces[1];

    await ClientsList.visit();
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    assert.equal(currentURL(), `/jobs?namespace=${namespace.name}`);
  });

  test('changing the namespace in the allocations hierarchy navigates to the jobs page', async function(assert) {
    const namespace = server.db.namespaces[1];
    const allocation = server.create('allocation', { job: server.db.jobs[0] });

    await Allocation.visit({ id: allocation.id });
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    assert.equal(currentURL(), `/jobs?namespace=${namespace.name}`);
  });

  test('changing the namespace in the storage hierarchy navigates to the volumes page', async function(assert) {
    const namespace = server.db.namespaces[1];

    await PluginsList.visit();
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    assert.equal(currentURL(), `/csi/volumes?namespace=${namespace.name}`);
  });

  test('changing the namespace refreshes the volumes list when on the volumes page', async function(assert) {
    const namespace = server.db.namespaces[1];

    await VolumesList.visit();

    let requests = server.pretender.handledRequests.filter(req =>
      req.url.startsWith('/v1/volumes')
    );
    assert.equal(requests.length, 1);
    assert.equal(requests[0].queryParams.namespace, undefined);

    // TODO: handle this with Page Objects
    await selectChoose('[data-test-namespace-switcher-parent]', namespace.name);

    requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/volumes'));
    assert.equal(requests.length, 2);
    assert.equal(requests[1].queryParams.namespace, namespace.name);
  });
});
