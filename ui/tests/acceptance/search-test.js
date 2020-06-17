import { module, test } from 'qunit';
import { click, currentURL, triggerEvent, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';

function getRequestCount(server, url) {
  return server.pretender.handledRequests.filterBy('url', url).length;
}

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches jobs and nodes with route-based caching and navigates to chosen items', async function(assert) {
    server.create('node', { name: 'xyz' });
    const otherNode = server.create('node', { name: 'aaa' });

    server.create('job', { id: 'vwxyz', namespaceId: 'default' });
    server.create('job', { id: 'xyz', namespace: 'default' });
    server.create('job', { id: 'abc', namespace: 'default' });

    await visit('/');

    let presearchJobsRequestCount = getRequestCount(server, '/v1/jobs');
    let presearchNodesRequestCount = getRequestCount(server, '/v1/nodes');

    await selectSearch(PageLayout.navbar.search.scope, 'xy');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.groups.length, 2);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (2)');
        assert.equal(jobs.options.length, 2);
        assert.equal(jobs.options[0].text, 'xyz');
        assert.equal(jobs.options[1].text, 'vwxyz');
      });

      search.groups[1].as(clients => {
        assert.equal(clients.name, 'Clients (1)');
        assert.equal(clients.options.length, 1);
        assert.equal(clients.options[0].text, 'xyz');
      });
    });

    assert.equal(
      presearchJobsRequestCount,
      getRequestCount(server, '/v1/jobs'),
      'no new jobs request should be sent when in the jobs hierarchy'
    );
    assert.equal(
      presearchNodesRequestCount + 1,
      getRequestCount(server, '/v1/nodes'),
      'a nodes request should happen when not in the clients hierarchy'
    );

    await PageLayout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    await selectSearch(PageLayout.navbar.search.scope, otherNode.id.substr(0, 3));

    await PageLayout.navbar.search.groups[1].options[0].click();
    assert.equal(currentURL(), `/clients/${otherNode.id}`);

    presearchJobsRequestCount = getRequestCount(server, '/v1/jobs');
    presearchNodesRequestCount = getRequestCount(server, '/v1/nodes');

    await selectSearch(PageLayout.navbar.search.scope, otherNode.id.substr(0, 3));

    assert.equal(
      presearchJobsRequestCount + 1,
      getRequestCount(server, '/v1/jobs'),
      'a jobs request should happen when not not in the jobs hierarchy'
    );
    assert.equal(
      presearchNodesRequestCount,
      getRequestCount(server, '/v1/nodes'),
      'no new nodes request should happen when in the clients hierarchy'
    );
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await PageLayout.navbar.search.click();

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });

  test('pressing slash starts a search', async function(assert) {
    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    await triggerEvent('.page-layout', 'keydown', {
      keyCode: 191, // slash
    });

    assert.ok(PageLayout.navbar.search.field.isPresent);
  });

  test('pressing slash when an input element is focused does not start a search', async function(assert) {
    server.create('node');
    server.create('job');

    await visit('/');

    assert.notOk(PageLayout.navbar.search.field.isPresent);

    // FIXME use page objects for this and below? 🤔
    await click('.search-box input');

    await triggerEvent('.search-box input', 'keydown', {
      keyCode: 191, // slash
    });

    assert.notOk(PageLayout.navbar.search.field.isPresent);
  });
});
