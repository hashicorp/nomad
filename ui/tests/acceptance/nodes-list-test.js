import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

function commonSetup() {
  const nodesCount = 5;
  const agentsCount = 3;
  server.createList('node', nodesCount);
  server.createList('agent', agentsCount);

  return { nodesCount, agentsCount };
}

function minimumSetup() {
  server.createList('node', 1);
  server.createList('agent', 1);
}

moduleForAcceptance('Acceptance | nodes list');

test('/nodes should have high level metrics for node types', function(assert) {
  const { nodesCount, agentsCount } = commonSetup();

  visit('/nodes');

  andThen(() => {
    assert.equal(find('.client-metric .title').text(), nodesCount);
    assert.equal(find('.server-metric .title').text(), agentsCount);
  });
});

test('/nodes should have a toggle to switch between clients and servers', function(assert) {
  commonSetup();

  visit('/nodes');

  click('.node-type-switcher a.servers');
  andThen(() => {
    assert.equal(currentURL(), '/nodes/servers');
  });

  click('.node-type-switcher a.clients');
  andThen(() => {
    assert.equal(currentURL(), '/nodes');
  });
});

test('/nodes should list all clients', function(assert) {
  const { nodesCount } = commonSetup();

  visit('/nodes');

  andThen(() => {
    assert.equal(find('.client-node-row').length, nodesCount);

    server.db.nodes.forEach((node, index) => {
      assert.equal(
        find(`.client-node-row:eq(${index}) td:eq(0)`).text(),
        node.id.split('-')[0],
        'Nodes are ordered'
      );
    });
  });
});

test('each client record should show high-level info of the client', function(assert) {
  minimumSetup();
  const node = server.db.nodes[0];

  visit('/nodes');

  andThen(() => {
    const nodeRow = find('.client-node-row:eq(0)');
    const allocations = server.db.allocations.where({ nodeId: node.id });

    assert.equal(nodeRow.find('td:eq(0)').text(), node.id.split('-')[0], 'ID');
    assert.equal(nodeRow.find('td:eq(1)').text(), node.name, 'Name');
    assert.equal(nodeRow.find('td:eq(2)').text(), node.status, 'Status');
    assert.equal(nodeRow.find('td:eq(3)').text(), node.http_addr.split(':')[0], 'Address');
    assert.equal(nodeRow.find('td:eq(4)').text(), node.http_addr.split(':')[1], 'Port');
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

test('/nodes/servers should list all servers', function(assert) {
  const { agentsCount } = commonSetup();

  visit('/nodes/servers');

  andThen(() => {
    assert.equal(find('.server-agent-row').length, agentsCount);

    server.db.agents.forEach((agent, index) => {
      assert.equal(
        find(`.server-agent-row:eq(${index}) td:eq(0)`).text(),
        agent.name,
        'Clients are ordered'
      );
    });
  });
});

test('each server should show high-level info of the server', function(assert) {
  minimumSetup();
  const agent = server.db.agents[0];

  visit('/nodes/servers');

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

  visit('/nodes/servers');
  click('.server-agent-row:eq(0)');

  andThen(() => {
    assert.equal(currentURL(), `/nodes/servers/${agent.name}`);
  });
});
