import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import ClientsList from 'nomad-ui/tests/pages/clients/list';

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

  ClientsList.visit();

  andThen(() => {
    assert.equal(ClientsList.nodes.length, pageSize);
    assert.ok(ClientsList.hasPagination, 'Pagination found on the page');

    const sortedNodes = server.db.nodes.sortBy('modifyIndex').reverse();

    ClientsList.nodes.forEach((node, index) => {
      assert.equal(node.id, sortedNodes[index].id.split('-')[0], 'Clients are ordered');
    });
  });
});

test('each client record should show high-level info of the client', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  ClientsList.visit();

  andThen(() => {
    const nodeRow = ClientsList.nodes.objectAt(0);
    const allocations = server.db.allocations.where({ nodeId: node.id });

    assert.equal(nodeRow.id, node.id.split('-')[0], 'ID');
    assert.equal(nodeRow.name, node.name, 'Name');
    assert.equal(nodeRow.status, node.status, 'Status');
    assert.equal(nodeRow.drain, node.drain + '', 'Draining');
    assert.equal(nodeRow.eligibility, node.schedulingEligibility, 'Eligibility');
    assert.equal(nodeRow.address, node.httpAddr);
    assert.equal(nodeRow.datacenter, node.datacenter, 'Datacenter');
    assert.equal(nodeRow.allocations, allocations.length, '# Allocations');
  });
});

test('each client should link to the client detail page', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  ClientsList.visit();

  andThen(() => {
    ClientsList.nodes.objectAt(0).clickRow();
  });

  andThen(() => {
    assert.equal(currentURL(), `/clients/${node.id}`);
  });
});

test('when there are no clients, there is an empty message', function(assert) {
  server.createList('agent', 1);

  ClientsList.visit();

  andThen(() => {
    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Clients');
  });
});

test('when there are clients, but no matches for a search term, there is an empty message', function(assert) {
  server.createList('agent', 1);
  server.create('node', { name: 'node' });

  ClientsList.visit();

  andThen(() => {
    ClientsList.search('client');
  });

  andThen(() => {
    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Matches');
  });
});

test('when accessing clients is forbidden, show a message with a link to the tokens page', function(assert) {
  server.create('agent');
  server.create('node', { name: 'node' });
  server.pretender.get('/v1/nodes', () => [403, {}, null]);

  ClientsList.visit();

  andThen(() => {
    assert.equal(ClientsList.error.title, 'Not Authorized');
  });

  andThen(() => {
    ClientsList.error.seekHelp();
  });

  andThen(() => {
    assert.equal(currentURL(), '/settings/tokens');
  });
});
