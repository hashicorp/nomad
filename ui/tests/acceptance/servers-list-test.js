import { click, find, findAll, currentURL, visit } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import { findLeader } from '../../mirage/config';

function minimumSetup() {
  server.createList('node', 1);
  server.createList('agent', 1);
}

moduleForAcceptance('Acceptance | servers list');

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

test('when accessing servers is forbidden, show a message with a link to the tokens page', function(assert) {
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
