import { click, find, findAll, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { findLeader } from '../../mirage/config';
import ipParts from 'nomad-ui/utils/ip-parts';

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
    const { address, port } = ipParts(node.httpAddr);

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
      nodeRow.querySelector('[data-test-client-address]').textContent.trim(),
      address,
      'Address'
    );
    assert.equal(nodeRow.querySelector('[data-test-client-port]').textContent.trim(), port, 'Port');
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

test('when there are clients, but no matches for a search term, there is an empty message', function(
  assert
) {
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

test('when accessing clients is forbidden, show a message with a link to the tokens page', function(
  assert
) {
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

test('/servers should list all servers', function(assert) {
  const agentsCount = 10;
  const pageSize = 8;

  server.createList('node', 1);
  server.createList('agent', agentsCount);

  const leader = findLeader(server.schema);

  visit('/servers');

  andThen(() => {
    assert.equal(findAll('[data-test-server-agent-row]').length, pageSize);

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
      const serverRow = findAll('[data-test-server-agent-row]')[agentNumber];
      assert.equal(
        serverRow.querySelector('[data-test-server-name]').textContent.trim(),
        sortedAgents[agentNumber].name,
        'Servers are ordered'
      );
    }
  });
});

test('each server should show high-level info of the server', function(assert) {
  minimumSetup();
  const agent = server.db.agents[0];

  visit('/servers');

  andThen(() => {
    const agentRow = find('[data-test-server-agent-row]');

    assert.equal(agentRow.querySelector('[data-test-server-name]').textContent, agent.name, 'Name');
    assert.equal(
      agentRow.querySelector('[data-test-server-status]').textContent,
      agent.status,
      'Status'
    );
    assert.equal(
      agentRow.querySelector('[data-test-server-is-leader]').textContent,
      'True',
      'Leader?'
    );
    assert.equal(
      agentRow.querySelector('[data-test-server-address]').textContent,
      agent.address,
      'Address'
    );
    assert.equal(
      agentRow.querySelector('[data-test-server-port]').textContent,
      agent.serf_port,
      'Serf Port'
    );
    assert.equal(
      agentRow.querySelector('[data-test-server-datacenter]').textContent,
      agent.tags.dc,
      'Datacenter'
    );
  });
});

test('each server should link to the server detail page', function(assert) {
  minimumSetup();
  const agent = server.db.agents[0];

  visit('/servers');
  andThen(() => {
    click('[data-test-server-agent-row]');
  });

  andThen(() => {
    assert.equal(currentURL(), `/servers/${agent.name}`);
  });
});

test('when accessing servers is forbidden, show a message with a link to the tokens page', function(
  assert
) {
  server.create('agent');
  server.pretender.get('/v1/agent/members', () => [403, {}, null]);

  visit('/servers');

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
