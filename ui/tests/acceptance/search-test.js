/* eslint-disable ember-a11y-testing/a11y-audit-called */ // TODO
import { module, skip, test } from 'qunit';
import { currentURL, triggerEvent, visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Layout from 'nomad-ui/tests/pages/layout';
import JobsList from 'nomad-ui/tests/pages/jobs/list';
import { selectSearch } from 'ember-power-select/test-support';
import sinon from 'sinon';
import Response from 'ember-cli-mirage/response';

import { COLLECTION_CACHE_DURATION } from 'nomad-ui/services/data-caches';

function getRequestCount(server, url) {
  return server.pretender.handledRequests.filterBy('url', url).length;
}

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search exposes and navigates to results from the fuzzy search endpoint', async function(assert) {
    server.create('node', { name: 'xyz' });
    const otherNode = server.create('node', { name: 'ghi' });

    server.create('job', { id: 'vwxyz', namespaceId: 'default', groupsCount: 1 });
    server.create('job', { id: 'xyz', name: 'xyz job', namespaceId: 'default', groupsCount: 1 });
    server.create('job', { id: 'abc', namespaceId: 'default', groupsCount: 1 });

    // Override random task group names to include job names to make results deterministic
    server.schema.taskGroups.all().models.forEach(group => {
      group.name = `${group.job.name}-group`;
      group.save();
    });

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'xy');

    Layout.navbar.search.as(search => {
      assert.equal(search.groups.length, 3);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (2)');
        assert.equal(jobs.options.length, 2);
        assert.equal(jobs.options[0].text, 'vwxyz');
        assert.equal(jobs.options[1].text, 'xyz job');
      });

      search.groups[1].as(clients => {
        assert.equal(clients.name, 'Clients (1)');
        assert.equal(clients.options.length, 1);
        assert.equal(clients.options[0].text, 'xyz');
      });

      search.groups[2].as(groups => {
        assert.equal(groups.name, 'Task Groups (2)');
        assert.equal(groups.options.length, 2);
        assert.equal(groups.options[0].text, 'vwxyz-group');
        assert.equal(groups.options[1].text, 'xyz job-group');
      });
    });

    await Layout.navbar.search.groups[0].options[1].click();
    assert.equal(currentURL(), '/jobs/xyz');

    await selectSearch(Layout.navbar.search.scope, otherNode.name);
    await Layout.navbar.search.groups[1].options[0].click();
    assert.equal(currentURL(), `/clients/${otherNode.id}`);

    await selectSearch(Layout.navbar.search.scope, 'xy');
    await Layout.navbar.search.groups[2].options[0].click();
    assert.equal(currentURL(), '/jobs/vwxyz/vwxyz-group');
  });

  test('search does not perform a request when only one character has been entered', async function(assert) {
    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'q');

    assert.ok(Layout.navbar.search.noOptionsShown);
    assert.equal(server.pretender.handledRequests.filterBy('url', '/v1/search/fuzzy').length, 1, 'expect the feature detection query');
  });

  test('when fuzzy search is disabled on the server, the search control is hidden', async function(assert) {
    server.post('/search/fuzzy', function() {
      return new Response(500, {}, '');
    });

    await visit('/');

    assert.ok(Layout.navbar.search.isHidden);
  });

  skip('search searches jobs and nodes with route- and time-based caching and navigates to chosen items', async function(assert) {
    server.create('node', { name: 'xyz' });
    const otherNode = server.create('node', { name: 'ghi' });

    server.create('job', { id: 'vwxyz', namespaceId: 'default' });
    server.create('job', { id: 'xyz', name: 'xyz job', namespace: 'default' });
    server.create('job', { id: 'abc', namespace: 'default' });

    await visit('/');

    const clock = sinon.useFakeTimers({
      now: new Date(),
      shouldAdvanceTime: true,
    });

    let presearchJobsRequestCount = getRequestCount(server, '/v1/jobs');
    let presearchNodesRequestCount = getRequestCount(server, '/v1/nodes');

    await selectSearch(Layout.navbar.search.scope, 'xy');

    Layout.navbar.search.as(search => {
      assert.equal(search.groups.length, 2);

      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (2)');
        assert.equal(jobs.options.length, 2);
        assert.equal(jobs.options[0].text, 'xyz job');
        assert.equal(jobs.options[1].text, 'vwxyz');
      });

      search.groups[1].as(clients => {
        assert.equal(clients.name, 'Clients (1)');
        assert.equal(clients.options.length, 1);
        assert.equal(clients.options[0].text, 'xyz');
      });
    });

    assert.equal(
      getRequestCount(server, '/v1/jobs'),
      presearchJobsRequestCount,
      'no new jobs request should be sent when in the jobs hierarchy'
    );
    assert.equal(
      getRequestCount(server, '/v1/nodes'),
      presearchNodesRequestCount + 1,
      'a nodes request should happen when not in the clients hierarchy'
    );

    await Layout.navbar.search.groups[0].options[0].click();
    assert.equal(currentURL(), '/jobs/xyz');

    await selectSearch(Layout.navbar.search.scope, otherNode.id.substr(0, 3));

    await Layout.navbar.search.groups[1].options[0].click();
    assert.equal(currentURL(), `/clients/${otherNode.id}`);

    presearchJobsRequestCount = getRequestCount(server, '/v1/jobs');
    presearchNodesRequestCount = getRequestCount(server, '/v1/nodes');

    await selectSearch(Layout.navbar.search.scope, 'zzzzzzzzzzz');

    assert.equal(
      getRequestCount(server, '/v1/jobs'),
      presearchJobsRequestCount,
      'a jobs request should not happen because the cache hasn’t expired'
    );
    assert.equal(
      presearchNodesRequestCount,
      getRequestCount(server, '/v1/nodes'),
      'no new nodes request should happen when in the clients hierarchy'
    );

    clock.tick(COLLECTION_CACHE_DURATION * 2);

    await selectSearch(Layout.navbar.search.scope, otherNode.id.substr(0, 3));

    assert.equal(
      getRequestCount(server, '/v1/jobs'),
      presearchJobsRequestCount + 1,
      'a jobs request should happen because the cache has expired'
    );

    clock.restore();
  });

  test('results are truncated at 10 per group', async function(assert) {
    server.create('node', { name: 'xyz' });

    for (let i = 0; i < 11; i++) {
      server.create('job', { id: `job-${i}`, namespaceId: 'default' });
    }

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'job');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (showing 10 of 11)');
        assert.equal(jobs.options.length, 10);
      });
    });
  });

  test('server-side truncation is indicated in the group label', async function(assert) {
    server.create('node', { name: 'xyz' });

    for (let i = 0; i < 21; i++) {
      server.create('job', { id: `job-${i}`, namespaceId: 'default' });
    }

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'job');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (showing 10 of 20+)');
      });
    });
  });

  skip('search highlights matching substrings', async function(assert) {
    server.create('node', { name: 'xyz' });

    server.create('job', { id: 'traefik', namespaceId: 'default' });
    server.create('job', { id: 'tracking', namespace: 'default' });
    server.create('job', { id: 'smtp-sensor', namespaceId: 'default' });

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'trae');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.options[0].text, 'traefik');
        assert.equal(jobs.options[0].formattedText, '*trae*fik');

        assert.equal(jobs.options[1].text, 'tracking');
        assert.equal(jobs.options[1].formattedText, '*tra*cking');
      });
    });

    await selectSearch(Layout.navbar.search.scope, 'ra');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.options[0].formattedText, 't*ra*efik');
        assert.equal(jobs.options[1].formattedText, 't*ra*cking');
      });
    });

    await selectSearch(Layout.navbar.search.scope, 'sensor');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.options[0].formattedText, '*s*mtp-*sensor*');
      });
    });
  });

  skip('results are truncated at 10 per group', async function(assert) {
    server.create('node', { name: 'xyz' });

    for (let i = 0; i < 15; i++) {
      server.create('job', { id: `job-${i}`, namespaceId: 'default' });
    }

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, 'job');

    Layout.navbar.search.as(search => {
      search.groups[0].as(jobs => {
        assert.equal(jobs.name, 'Jobs (showing 10 of 15)');
        assert.equal(jobs.options.length, 10);
      });
    });
  });

  skip('node id prefix matches take priority over node name matches', async function(assert) {
    const nodeToMatchById = server.create('node', { name: 'xyz' });

    const idPrefix = nodeToMatchById.id.substr(0, 5);

    const nodeToMatchByName = server.create('node', {
      name: `node-name-with-id-piece-${idPrefix}`,
    });

    await visit('/');

    await selectSearch(Layout.navbar.search.scope, idPrefix);

    Layout.navbar.search.as(search => {
      search.groups[1].as(clients => {
        assert.equal(clients.options[0].text, nodeToMatchById.name);
        assert.equal(clients.options[1].text, nodeToMatchByName.name);
      });
    });
  });

  test('clicking the search field starts search immediately', async function(assert) {
    await visit('/');

    assert.notOk(Layout.navbar.search.field.isPresent);

    await Layout.navbar.search.click();

    assert.ok(Layout.navbar.search.field.isPresent);
  });

  test('pressing slash starts a search', async function(assert) {
    await visit('/');

    assert.notOk(Layout.navbar.search.field.isPresent);

    await triggerEvent('.page-layout', 'keydown', {
      keyCode: 191, // slash
    });

    assert.ok(Layout.navbar.search.field.isPresent);
  });

  test('pressing slash when an input element is focused does not start a search', async function(assert) {
    server.create('node');
    server.create('job');

    await visit('/');

    assert.notOk(Layout.navbar.search.field.isPresent);

    await JobsList.search.click();
    await JobsList.search.keydown({ keyCode: 191 });

    assert.notOk(Layout.navbar.search.field.isPresent);
  });
});
