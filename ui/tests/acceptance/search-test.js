import { module, test } from 'qunit';
import { currentURL, triggerKeyEvent, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';
import Response from 'ember-cli-mirage/response';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches and choosing an item navigates to it', async function(assert) {
    const node = server.create('node');
    const job = server.create('job', { id: 'xyz', namespaceId: 'default' });

    await visit('/');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 1);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (1)');
        assert.equal(jobs.options.length, 1);
        assert.equal(jobs.options[0].text, 'xyz');
        assert.equal(jobs.options[0].statusClass, job.status);
      });
    });

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    const allocation = server.db.allocations.firstObject;
    await selectSearch(PageLayout.navbar.search.scope, allocation.id.substr(0, 3));

    assert.equal(PageLayout.navbar.search.groups[0].name, 'Allocations (1)');
    assert.equal(
      PageLayout.navbar.search.groups[0].options[0].statusClass,
      allocation.clientStatus
    );

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/allocations/${allocation.id}`);

    await selectSearch(PageLayout.navbar.search.scope, node.id.substr(0, 3));

    assert.equal(PageLayout.navbar.search.groups[0].name, 'Clients (1)');
    assert.equal(PageLayout.navbar.search.groups[0].options[0].statusClass, node.status);

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('only allocation, client, and job search results show', async function(assert) {
    server.create('node');
    server.create('job', { id: 'xyz', namespaceId: 'default' });

    await visit('/');

    const evaluation = server.db.evaluations.firstObject;
    await selectSearch(PageLayout.navbar.search.scope, evaluation.id.substr(0, 3));

    assert.ok(PageLayout.navbar.search.hasNoMatches);
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });

  test('region is included in the search query if present', async function(assert) {
    const done = assert.async();

    server.create('region', { id: 'global' });
    const region = server.create('region', { id: 'region-2' });

    server.post('/search', (server, { queryParams }) => {
      assert.equal(queryParams.region, region.id);
      done();
    });

    await visit(`/?region=${region.id}`);
    await selectSearch(PageLayout.navbar.search.scope, 'string');
  });

  test('namespace is included in the search query if present', async function(assert) {
    const done = assert.async();

    server.create('namespace');
    const namespace = server.create('namespace');

    server.post('/search', (server, { queryParams }) => {
      assert.equal(queryParams.namespace, namespace.id);
      done();
    });

    await visit(`/jobs?namespace=${namespace.id}`);
    await selectSearch(PageLayout.navbar.search.scope, 'string');
  });

  test('namespace is included when fetching and rendering job search results', async function(assert) {
    server.create('namespace');
    const namespace = server.create('namespace');
    const job = server.create('job', {
      id: 'xyz',
      namespaceId: namespace.id,
      createAllocations: false,
    });

    await visit(`/?namespace=${namespace.id}`);
    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 1);

      search.groups[0].as(jobs => {
        assert.equal(jobs.options[0].text, 'xyz');
        assert.equal(jobs.options[0].statusClass, job.status);
      });
    });

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), `/jobs/xyz?namespace=${namespace.id}`);
  });

  test('an error when searching is treated as no results', async function(assert) {
    server.post('/search', () => {
      return new Response(500, {}, 'no such file or directory');
    });

    await visit('/');
    await selectSearch(PageLayout.navbar.search.scope, 'abc');

    assert.ok(PageLayout.navbar.search.hasNoMatches);
    // TODO is this sensible?
  });
});
