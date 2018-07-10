import { click, find, findAll, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

function minimumSetup() {
  server.createList('node', 1);
  server.createList('agent', 1);
}

moduleForAcceptance('Acceptance | clients list');

test('/clients should list one page of clients', function(assert) {
  // Make sure to make more nodes than 1 page to assert that pagination is working
  const nodesCount = 10;
  const pageSize = 8;

  server.createList('node', nodesCount);
  server.createList('agent', 1);

  visit('/clients');

  andThen(() => {
    assert.equal(findAll('[data-test-client-node-row]').length, pageSize);
    assert.ok(find('[data-test-pagination]'), 'Pagination found on the page');

    const sortedNodes = server.db.nodes.sortBy('modifyIndex').reverse();

    for (var nodeNumber = 0; nodeNumber < pageSize; nodeNumber++) {
      const nodeRow = findAll('[data-test-client-node-row]')[nodeNumber];
      assert.equal(
        nodeRow.querySelector('[data-test-client-id]').textContent.trim(),
        sortedNodes[nodeNumber].id.split('-')[0],
        'Clients are ordered'
      );
    }
  });
});

test('each client record should show high-level info of the client', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  visit('/clients');

  andThen(() => {
    const nodeRow = find('[data-test-client-node-row]');
    const allocations = server.db.allocations.where({ nodeId: node.id });

    assert.equal(
      nodeRow.querySelector('[data-test-client-id]').textContent.trim(),
      node.id.split('-')[0],
      'ID'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-name]').textContent.trim(),
      node.name,
      'Name'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-status]').textContent.trim(),
      node.status,
      'Status'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-drain]').textContent.trim(),
      node.drain + '',
      'Draining'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-eligibility]').textContent.trim(),
      node.schedulingEligibility,
      'Eligibility'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-address]').textContent.trim(),
      node.httpAddr
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-datacenter]').textContent.trim(),
      node.datacenter,
      'Datacenter'
    );
    assert.equal(
      nodeRow.querySelector('[data-test-client-allocations]').textContent.trim(),
      allocations.length,
      '# Allocations'
    );
  });
});

test('each client should link to the client detail page', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  visit('/clients');
  andThen(() => {
    click('[data-test-client-node-row]');
  });

  andThen(() => {
    assert.equal(currentURL(), `/clients/${node.id}`);
  });
});

test('when there are no clients, there is an empty message', function(assert) {
  server.createList('agent', 1);

  visit('/clients');

  andThen(() => {
    assert.ok(find('[data-test-empty-clients-list]'));
    assert.equal(find('[data-test-empty-clients-list-headline]').textContent, 'No Clients');
  });
});

test('when there are clients, but no matches for a search term, there is an empty message', function(assert) {
  server.createList('agent', 1);
  server.create('node', { name: 'node' });

  visit('/clients');

  andThen(() => {
    fillIn('.search-box input', 'client');
  });

  andThen(() => {
    assert.ok(find('[data-test-empty-clients-list]'));
    assert.equal(find('[data-test-empty-clients-list-headline]').textContent, 'No Matches');
  });
});

test('when accessing clients is forbidden, show a message with a link to the tokens page', function(assert) {
  server.create('agent');
  server.create('node', { name: 'node' });
  server.pretender.get('/v1/nodes', () => [403, {}, null]);

  visit('/clients');

  andThen(() => {
    assert.equal(find('[data-test-error-title]').textContent, 'Not Authorized');
  });

  andThen(() => {
    click('[data-test-error-message] a');
  });

  andThen(() => {
    assert.equal(currentURL(), '/settings/tokens');
  });
});
