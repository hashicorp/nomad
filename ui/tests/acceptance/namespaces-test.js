import { visit, find, findAll, click } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

moduleForAcceptance('Acceptance | namespaces (disabled)', {
  beforeEach() {
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  },
});

test('the namespace switcher is not in the gutter menu', function(assert) {
  visit('/jobs');

  andThen(() => {
    assert.notOk(find('.gutter .menu .namespace-switcher'), 'No namespace switcher found');
  });
});

test('the jobs request is made with no query params', function(assert) {
  visit('/jobs');

  andThen(() => {
    const request = server.pretender.handledRequests.findBy('url', '/v1/jobs');
    assert.equal(request.queryParams.namespace, undefined, 'No namespace query param');
  });
});

moduleForAcceptance('Acceptance | namespaces (enabled)', {
  beforeEach() {
    server.createList('namespace', 3);
    server.create('agent');
    server.create('node');
    server.createList('job', 5);
  },
});

test('the namespace switcher lists all namespaces', function(assert) {
  const namespaces = server.db.namespaces;

  visit('/jobs');

  andThen(() => {
    assert.ok(find('[data-test-namespace-switcher]'), 'Namespace switcher found');
  });

  andThen(() => {
    click('[data-test-namespace-switcher] .ember-power-select-trigger');
  });

  andThen(() => {
    // TODO this selector should be scoped to only the namespace switcher options,
    // but ember-wormhole makes that difficult.
    assert.equal(
      findAll('.ember-power-select-option').length,
      namespaces.length,
      'All namespaces are in the switcher'
    );
    assert.equal(
      find('.ember-power-select-option').textContent.trim(),
      'Default Namespace',
      'The first namespace is always the default one'
    );

    namespaces
      .slice(1)
      .sortBy('name')
      .forEach((namespace, index) => {
        assert.equal(
          findAll('.ember-power-select-option')[index + 1].textContent.trim(),
          namespace.name,
          `index ${index + 1}: ${namespace.name}`
        );
      });
  });
});

test('changing the namespace sets the namespace in localStorage', function(assert) {
  const namespace = server.db.namespaces[1];

  visit('/jobs');

  selectChoose('[data-test-namespace-switcher]', namespace.name);
  andThen(() => {
    assert.equal(
      window.localStorage.nomadActiveNamespace,
      namespace.id,
      'Active namespace was set'
    );
  });
});

test('changing the namespace refreshes the jobs list when on the jobs page', function(assert) {
  const namespace = server.db.namespaces[1];

  visit('/jobs');

  andThen(() => {
    const requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 1, 'First request to jobs');
    assert.equal(
      requests[0].queryParams.namespace,
      undefined,
      'Namespace query param is defaulted to "default"/undefined'
    );
  });

  selectChoose('[data-test-namespace-switcher]', namespace.name);

  andThen(() => {
    const requests = server.pretender.handledRequests.filter(req => req.url.startsWith('/v1/jobs'));
    assert.equal(requests.length, 2, 'Second request to jobs');
    assert.equal(
      requests[1].queryParams.namespace,
      namespace.name,
      'Namespace query param on second request'
    );
  });
});
