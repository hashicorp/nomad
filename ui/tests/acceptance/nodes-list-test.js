import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { findLeader } from '../../mirage/config';
import ipParts from 'nomad-ui/utils/ip-parts';

function minimumSetup() {
  server.createList('node', 1);
  server.createList('agent', 1);
}

moduleForAcceptance('Acceptance | nodes list');

test('/nodes should list one page of clients', function(assert) {
  // Make sure to make more nodes than 1 page to assert that pagination is working
  const nodesCount = 10;
  const pageSize = 8;

  server.createList('node', nodesCount);
  server.createList('agent', 1);

  visit('/nodes');

  andThen(() => {
    assert.equal(find('.client-node-row').length, pageSize);
    assert.ok(find('.pagination').length, 'Pagination found on the page');

    const sortedNodes = server.db.nodes.sortBy('modifyIndex').reverse();

    for (var nodeNumber = 0; nodeNumber < pageSize; nodeNumber++) {
      assert.equal(
        find(`.client-node-row:eq(${nodeNumber}) td:eq(0)`).text(),
        sortedNodes[nodeNumber].id.split('-')[0],
        'Nodes are ordered'
      );
    }
  });
});

test('each client record should show high-level info of the client', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  visit('/nodes');

  andThen(() => {
    const nodeRow = find('.client-node-row:eq(0)');
    const allocations = server.db.allocations.where({ nodeId: node.id });
    const { address, port } = ipParts(node.httpAddr);

    assert.equal(nodeRow.find('td:eq(0)').text(), node.id.split('-')[0], 'ID');
    assert.equal(nodeRow.find('td:eq(1)').text(), node.name, 'Name');
    assert.equal(nodeRow.find('td:eq(2)').text(), node.status, 'Status');
    assert.equal(nodeRow.find('td:eq(3)').text(), address, 'Address');
    assert.equal(nodeRow.find('td:eq(4)').text(), port, 'Port');
    assert.equal(nodeRow.find('td:eq(5)').text(), node.datacenter, 'Datacenter');
    assert.equal(nodeRow.find('td:eq(6)').text(), allocations.length, '# Allocations');
  });
});

test('each client should link to the client detail page', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  visit('/nodes');
  click('.client-node-row:eq(0)');

  andThen(() => {
    assert.equal(currentURL(), `/nodes/${node.id}`);
  });
});

test('/servers should list all servers', function(assert) {
  const agentsCount = 10;
  const pageSize = 8;

  server.createList('node', 1);
  server.createList('agent', agentsCount);

  const leader = findLeader(server.schema);

  visit('/servers');

  andThen(() => {
    assert.equal(find('.server-agent-row').length, pageSize);

    const sortedAgents = server.db.agents
      .sort((a, b) => {
        if (`${a.address}:${a.tags.port}` === leader) {
          return 1;
        } else if (`${b.address}:${b.tags.port}` === leader) {
          return -1;
        }
        return 0;
      })
      .reverse();

    for (var agentNumber = 0; agentNumber < 8; agentNumber++) {
      assert.equal(
        find(`.server-agent-row:eq(${agentNumber}) td:eq(0)`).text(),
        sortedAgents[agentNumber].name,
        'Clients are ordered'
      );
    }
  });
});

test('each server should show high-level info of the server', function(assert) {
  minimumSetup();
  const agent = server.db.agents[0];

  visit('/servers');

  andThen(() => {
    const agentRow = find('.server-agent-row:eq(0)');

    assert.equal(agentRow.find('td:eq(0)').text(), agent.name, 'Name');
    assert.equal(agentRow.find('td:eq(1)').text(), agent.status, 'Status');
    assert.equal(agentRow.find('td:eq(2)').text(), 'True', 'Leader?');
    assert.equal(agentRow.find('td:eq(3)').text(), agent.address, 'Address');
    assert.equal(agentRow.find('td:eq(4)').text(), agent.serf_port, 'Serf Port');
    assert.equal(agentRow.find('td:eq(5)').text(), agent.tags.dc, 'Datacenter');
  });
});

test('each server should link to the server detail page', function(assert) {
  minimumSetup();
  const agent = server.db.agents[0];

  visit('/servers');
  click('.server-agent-row:eq(0)');

  andThen(() => {
    assert.equal(currentURL(), `/servers/${agent.name}`);
  });
});
