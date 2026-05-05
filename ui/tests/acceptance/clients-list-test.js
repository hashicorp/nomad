/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { currentURL, settled } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import ClientsList from 'nomad-ui/tests/pages/clients/list';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

module('Acceptance | clients list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    this.server.createList('node-pool', 3);
  });

  test('it passes an accessibility audit', async function (assert) {
    const nodesCount = ClientsList.pageSize + 1;

    this.server.createList('node', nodesCount);
    this.server.createList('agent', 1);

    await ClientsList.visit();
    await a11yAudit(assert);
  });

  test('/clients should list one page of clients', async function (assert) {
    faker.seed(1);
    // Make sure to make more nodes than 1 page to assert that pagination is working
    const nodesCount = ClientsList.pageSize + 1;
    this.server.createList('node', nodesCount);
    this.server.createList('agent', 1);

    await ClientsList.visit();

    await percySnapshot(assert);

    assert.deepEqual(ClientsList.nodes.length, ClientsList.pageSize);
    assert.ok(ClientsList.hasPagination, 'Pagination found on the page');

    const sortedNodes = this.server.db.nodes.sortBy('modifyIndex').reverse();

    ClientsList.nodes.forEach((node, index) => {
      assert.deepEqual(
        node.id,
        sortedNodes[index].id.split('-')[0],
        'Clients are ordered',
      );
    });

    assert.ok(getPageTitle().includes('Clients'));
  });

  test('each client record should show high-level info of the client', async function (assert) {
    const node = this.server.create('node', 'draining', {
      status: 'ready',
    });

    this.server.createList('agent', 1);

    await ClientsList.visit();

    const nodeRow = ClientsList.nodes.objectAt(0);
    const allocations = this.server.db.allocations.where({ nodeId: node.id });

    assert.deepEqual(nodeRow.id, node.id.split('-')[0], 'ID');
    assert.deepEqual(nodeRow.name, node.name, 'Name');
    assert.deepEqual(nodeRow.nodePool, node.nodePool, 'Node Pool');
    assert.deepEqual(
      nodeRow.compositeStatus.text,
      'Ready Ineligible Draining',
      'Combined status, draining, and eligbility',
    );
    assert.deepEqual(nodeRow.address, node.httpAddr);
    assert.deepEqual(nodeRow.datacenter, node.datacenter, 'Datacenter');
    assert.deepEqual(nodeRow.version, node.version, 'Version');
    assert.strictEqual(
      Number(nodeRow.allocations),
      allocations.length,
      '# Allocations',
    );
  });

  test('each client record should show running allocations', async function (assert) {
    this.server.createList('agent', 1);

    const node = this.server.create('node', {
      modifyIndex: 4,
      status: 'ready',
      schedulingEligibility: 'eligible',
      drain: false,
    });

    this.server.create('job', { createAllocations: false });

    const running = this.server.createList('allocation', 2, {
      clientStatus: 'running',
    });
    this.server.createList('allocation', 3, { clientStatus: 'pending' });
    this.server.createList('allocation', 10, { clientStatus: 'complete' });

    await ClientsList.visit();

    const nodeRow = ClientsList.nodes.objectAt(0);

    assert.deepEqual(nodeRow.id, node.id.split('-')[0], 'ID');
    assert.deepEqual(
      nodeRow.compositeStatus.text,
      'Ready Eligible Not Draining',
      'Combined status, draining, and eligbility',
    );
    assert.strictEqual(
      Number(nodeRow.allocations),
      running.length,
      '# Allocations',
    );
  });

  test('client status, draining, and eligibility are combined into one column that stays sorted on status', async function (assert) {
    this.server.createList('agent', 1);

    this.server.create('node', {
      modifyIndex: 5,
      status: 'ready',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    this.server.create('node', {
      modifyIndex: 4,
      status: 'initializing',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    this.server.create('node', {
      modifyIndex: 3,
      status: 'down',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    this.server.create('node', {
      modifyIndex: 2,
      status: 'down',
      schedulingEligibility: 'ineligible',
      drain: false,
    });
    this.server.create('node', {
      modifyIndex: 1,
      status: 'ready',
      schedulingEligibility: 'ineligible',
      drain: false,
    });
    this.server.create('node', 'draining', {
      schedulingEligibility: 'eligible',
      modifyIndex: 0,
      status: 'ready',
    });

    await ClientsList.visit();
    assert.deepEqual(
      ClientsList.nodes[0].compositeStatus.text,
      'Ready Eligible Not Draining',
    );
    assert.deepEqual(
      ClientsList.nodes[1].compositeStatus.text,
      'Initializing Eligible Not Draining',
    );
    assert.deepEqual(
      ClientsList.nodes[2].compositeStatus.text,
      'Down Eligible Not Draining',
    );
    assert.deepEqual(
      ClientsList.nodes[3].compositeStatus.text,
      'Down Ineligible Not Draining',
    );
    assert.deepEqual(
      ClientsList.nodes[4].compositeStatus.text,
      'Ready Ineligible Not Draining',
    );
    assert.deepEqual(
      ClientsList.nodes[5].compositeStatus.text,
      'Ready Eligible Draining',
    );

    await ClientsList.sortBy('status');

    assert.deepEqual(
      ClientsList.nodes.map((n) => n.compositeStatus.text),
      [
        'Ready Eligible Draining',
        'Ready Ineligible Not Draining',
        'Ready Eligible Not Draining',
        'Initializing Eligible Not Draining',
        'Down Ineligible Not Draining',
        'Down Eligible Not Draining',
      ],
      'Nodes are sorted only by status, and otherwise default to modifyIndex',
    );

    // Simulate a client state change arriving through polling
    let discoClient = this.owner
      .lookup('service:store')
      .peekAll('node')
      .findBy('modifyIndex', 5);
    discoClient.set('status', 'disconnected');

    await settled();

    assert.deepEqual(
      ClientsList.nodes.map((n) => n.compositeStatus.text),
      [
        'Ready Eligible Draining',
        'Ready Ineligible Not Draining',
        'Disconnected Eligible Not Draining',
        'Initializing Eligible Not Draining',
        'Down Ineligible Not Draining',
        'Down Eligible Not Draining',
      ],
    );
  });

  test('each client should link to the client detail page', async function (assert) {
    this.server.createList('node', 1);
    this.server.createList('agent', 1);

    const node = this.server.db.nodes[0];

    await ClientsList.visit();
    await ClientsList.nodes.objectAt(0).clickRow();

    assert.deepEqual(currentURL(), `/clients/${node.id}`);
  });

  test('when there are no clients, there is an empty message', async function (assert) {
    faker.seed(1);
    this.server.createList('agent', 1);

    await ClientsList.visit();

    await percySnapshot(assert);

    assert.ok(ClientsList.isEmpty);
    assert.deepEqual(ClientsList.empty.headline, 'No Clients');
  });

  test('when there are clients, but no matches for a search term, there is an empty message', async function (assert) {
    this.server.createList('agent', 1);
    this.server.create('node', { name: 'node' });

    await ClientsList.visit();

    await ClientsList.search('client');
    assert.ok(ClientsList.isEmpty);
    assert.deepEqual(ClientsList.empty.headline, 'No Matches');
  });

  test('when accessing clients is forbidden, show a message with a link to the tokens page', async function (assert) {
    this.server.create('agent');
    this.server.create('node', { name: 'node' });
    this.server.pretender.get('/v1/nodes', () => [403, {}, null]);

    await ClientsList.visit();

    assert.deepEqual(ClientsList.error.title, 'Not Authorized');

    await ClientsList.error.seekHelp();

    assert.deepEqual(currentURL(), '/settings/tokens');
  });

  pageSizeSelect({
    resourceName: 'client',
    pageObject: ClientsList,
    pageObjectList: ClientsList.nodes,
    async setup() {
      this.server.createList('node', ClientsList.pageSize);
      this.server.createList('agent', 1);
      await ClientsList.visit();
    },
  });

  testFacet('Class', {
    facet: ClientsList.facets.class,
    paramName: 'class',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('nodeClass'))).sort();
    },
    async beforeEach() {
      this.server.create('agent');
      this.server.createList('node', 2, { nodeClass: 'nc-one' });
      this.server.createList('node', 2, { nodeClass: 'nc-two' });
      this.server.createList('node', 2, { nodeClass: 'nc-three' });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.nodeClass),
  });

  testFacet('State', {
    facet: ClientsList.facets.state,
    paramName: 'state',
    expectedOptions: [
      'initializing',
      'ready',
      'down',
      'disconnected',
      'eligible',
      'ineligible',
      'draining',
      'not draining',
    ],
    async beforeEach() {
      this.server.create('agent');

      this.server.createList('node', 2, { status: 'initializing' });
      this.server.createList('node', 2, { status: 'ready' });
      this.server.createList('node', 2, { status: 'down' });

      this.server.createList('node', 2, {
        schedulingEligibility: 'eligible',
        drain: false,
      });
      this.server.createList('node', 2, {
        schedulingEligibility: 'ineligible',
        drain: false,
      });
      this.server.createList('node', 2, {
        schedulingEligibility: 'ineligible',
        drain: true,
      });

      await ClientsList.visit();
    },
    filter: (node, selection) => {
      if (selection.includes('draining') && !node.drain) return false;
      if (
        selection.includes('ineligible') &&
        node.schedulingEligibility === 'eligible'
      )
        return false;

      return !selection.includes(node.status);
    },
  });

  testFacet('Node Pools', {
    facet: ClientsList.facets.nodePools,
    paramName: 'nodePool',
    expectedOptions() {
      return this.server.db.nodePools
        .filter((p) => p.name !== 'all') // The node pool 'all' should not be a filter.
        .map((p) => p.name);
    },
    async beforeEach() {
      this.server.create('agent');
      this.server.create('node-pool', { name: 'all' });
      this.server.create('node-pool', { name: 'default' });
      this.server.createList('node-pool', 10);

      // Make sure each node pool has at least one node.
      this.server.db.nodePools.forEach((p) => {
        this.server.createList('node', 2, { nodePool: p.name });
      });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.nodePool),
  });

  testFacet('Datacenters', {
    facet: ClientsList.facets.datacenter,
    paramName: 'dc',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('datacenter'))).sort();
    },
    async beforeEach() {
      this.server.create('agent');
      this.server.createList('node', 2, { datacenter: 'pdx-1' });
      this.server.createList('node', 2, { datacenter: 'nyc-1' });
      this.server.createList('node', 2, { datacenter: 'ams-1' });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.datacenter),
  });

  testFacet('Versions', {
    facet: ClientsList.facets.version,
    paramName: 'version',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('version'))).sort();
    },
    async beforeEach() {
      this.server.create('agent');
      this.server.createList('node', 2, { version: '0.12.0' });
      this.server.createList('node', 2, { version: '1.1.0-beta1' });
      this.server.createList('node', 2, { version: '1.2.0+ent' });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.version),
  });

  testFacet('Volumes', {
    facet: ClientsList.facets.volume,
    paramName: 'volume',
    expectedOptions(nodes) {
      const flatten = (acc, val) => acc.concat(Object.keys(val));
      return Array.from(
        new Set(nodes.mapBy('hostVolumes').reduce(flatten, [])),
      );
    },
    async beforeEach() {
      this.server.create('agent');
      this.server.createList('node', 2, {
        hostVolumes: { One: { Name: 'One' } },
      });
      this.server.createList('node', 2, {
        hostVolumes: { One: { Name: 'One' }, Two: { Name: 'Two' } },
      });
      this.server.createList('node', 2, {
        hostVolumes: { Two: { Name: 'Two' } },
      });
      await ClientsList.visit();
    },
    filter: (node, selection) =>
      Object.keys(node.hostVolumes).find((volume) =>
        selection.includes(volume),
      ),
  });

  test('when the facet selections result in no matches, the empty state states why', async function (assert) {
    this.server.create('agent');
    this.server.createList('node', 2, { status: 'ready' });

    await ClientsList.visit();
    await ClientsList.facets.state.toggle();
    await ClientsList.facets.state.options.objectAt(1).toggle();
    assert.ok(ClientsList.isEmpty, 'There is an empty message');
    assert.deepEqual(
      ClientsList.empty.headline,
      'No Matches',
      'The message is appropriate',
    );
  });

  test('the clients list is immediately filtered based on query params', async function (assert) {
    this.server.create('agent');
    this.server.create('node', { nodeClass: 'omg-large' });
    this.server.create('node', { nodeClass: 'wtf-tiny' });

    await ClientsList.visit({ class: JSON.stringify(['wtf-tiny']) });

    assert.deepEqual(
      ClientsList.nodes.length,
      1,
      'Only one client shown due to query param',
    );
  });

  function testFacet(
    label,
    { facet, paramName, beforeEach, filter, expectedOptions },
  ) {
    test(`the ${label} facet has the correct options`, async function (assert) {
      await beforeEach.call(this);
      await facet.toggle();

      let expectation;
      if (typeof expectedOptions === 'function') {
        expectation = expectedOptions.call(this, this.server.db.nodes);
      } else {
        expectation = expectedOptions;
      }

      assert.deepEqual(
        facet.options.map((option) => {
          return option.key.trim();
        }),
        expectation,
        'Options for facet are as expected',
      );
    });

    test(`the ${label} facet filters the nodes list by ${label}`, async function (assert) {
      let option;

      await beforeEach.call(this);

      await facet.toggle();
      option = facet.options.objectAt(0);
      await option.toggle();

      const selection = [option.key];
      const expectedNodes = this.server.db.nodes
        .filter((node) => filter(node, selection))
        .sortBy('modifyIndex')
        .reverse();

      ClientsList.nodes.forEach((node, index) => {
        assert.deepEqual(
          node.id,
          expectedNodes[index].id.split('-')[0],
          `Node at ${index} is ${expectedNodes[index].id}`,
        );
      });
    });

    test(`selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
      const selection = [];

      await beforeEach.call(this);
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const expectedNodes = this.server.db.nodes
        .filter((node) => filter(node, selection))
        .sortBy('modifyIndex')
        .reverse();

      ClientsList.nodes.forEach((node, index) => {
        assert.deepEqual(
          node.id,
          expectedNodes[index].id.split('-')[0],
          `Node at ${index} is ${expectedNodes[index].id}`,
        );
      });
    });

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
      const selection = [];

      await beforeEach.call(this);
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      // State is different from the other facets, in that it is an "exclusive" filter, whete others are "inclusive".
      // Because of this, it doesn't pass "state" as a stringified-array query param; rather, exclusion is indicated
      // for each option with a "${optionName}=false" query param.

      const stateString = `/clients?${selection
        .map((option) => `state_${option}=false`)
        .join('&')}`;
      const nonStateString = `/clients?${paramName}=${encodeURIComponent(
        JSON.stringify(selection),
      )}`;

      assert.deepEqual(
        currentURL(),
        paramName === 'state' ? stateString : nonStateString,
        'URL has the correct query param key and value',
      );
    });
  }
});
