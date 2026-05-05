/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import { findLeader } from '../../mirage/config';
import ServersList from 'nomad-ui/tests/pages/servers/list';
import formatHost from 'nomad-ui/utils/format-host';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

const minimumSetup = (server) => {
  faker.seed(1);
  server.createList('node-pool', 1);
  server.createList('node', 1);
  server.createList('agent', 1);
};

const agentSort = (leader) => (a, b) => {
  if (formatHost(a.member.Address, a.member.Tags.port) === leader) {
    return 1;
  } else if (formatHost(b.member.Address, b.member.Tags.port) === leader) {
    return -1;
  }
  return 0;
};

module('Acceptance | servers list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    this.server.create('region', { id: 'global' });
  });

  test('it passes an accessibility audit', async function (assert) {
    minimumSetup(this.server);
    await ServersList.visit();
    await a11yAudit(assert);
  });

  test('/servers should list all servers', async function (assert) {
    faker.seed(1);
    this.server.createList('node-pool', 1);
    this.server.createList('node', 1);
    this.server.createList('agent', 10);

    const leader = findLeader(this.server.schema);
    const sortedAgents = this.server.db.agents
      .sort(agentSort(leader))
      .reverse();

    await ServersList.visit();
    await percySnapshot(assert);

    assert.deepEqual(
      ServersList.servers.length,
      ServersList.pageSize,
      'List is stopped at pageSize',
    );

    ServersList.servers.forEach((server, index) => {
      assert.deepEqual(
        server.name,
        sortedAgents[index].name,
        'Servers are ordered',
      );
    });

    assert.ok(getPageTitle().includes('Servers'));
  });

  test('each server should show high-level info of the server', async function (assert) {
    minimumSetup(this.server);
    const agent = this.server.db.agents[0];

    await ServersList.visit();

    const agentRow = ServersList.servers.objectAt(0);

    assert.deepEqual(agentRow.name, agent.name, 'Name');
    assert.deepEqual(
      agentRow.status,
      agent.member.Status[0].toUpperCase() + agent.member.Status.substring(1),
      'Status',
    );
    assert.deepEqual(agentRow.leader, 'True', 'Leader?');
    assert.deepEqual(agentRow.address, agent.member.Address, 'Address');
    assert.strictEqual(
      Number(agentRow.serfPort),
      agent.member.Port,
      'Serf Port',
    );
    assert.deepEqual(agentRow.datacenter, agent.member.Tags.dc, 'Datacenter');
    assert.deepEqual(agentRow.version, agent.version, 'Version');
  });

  test('each server should link to the server detail page', async function (assert) {
    minimumSetup(this.server);
    const agent = this.server.db.agents[0];

    await ServersList.visit();
    await ServersList.servers.objectAt(0).clickRow();

    assert.deepEqual(
      currentURL(),
      `/servers/${agent.name}`,
      'Now at the server detail page',
    );
  });

  test('when accessing servers is forbidden, show a message with a link to the tokens page', async function (assert) {
    this.server.create('agent');
    this.server.pretender.get('/v1/agent/members', () => [403, {}, null]);

    await ServersList.visit();
    assert.deepEqual(ServersList.error.title, 'Not Authorized');

    await ServersList.error.seekHelp();
    assert.deepEqual(currentURL(), '/settings/tokens');
  });

  test('multiple regions should each show leadership values', async function (assert) {
    this.server.createList('node-pool', 1);
    this.server.createList('node', 1);
    this.server.create('region', { id: 'global' });
    this.server.create('region', { id: 'galactic' });
    this.server.createList('agent', 3);
    this.server.db.agents[0].member.Tags.region = 'global';
    this.server.db.agents[1].member.Tags.region = 'galactic';
    this.server.db.agents[2].member.Tags.region = 'galactic';
    await ServersList.visit();
    assert.deepEqual(
      ServersList.servers.objectAt(0).leader,
      'True (galactic)',
      'Leadership is shown for the galactic region',
    );
    assert.deepEqual(
      ServersList.servers.objectAt(1).leader,
      'True (global)',
      'Leadership is shown for the global region',
    );
    assert.deepEqual(
      ServersList.servers.objectAt(2).leader,
      'False',
      'Non-leader servers are shown',
    );
  });
});
