import { currentURL, settled } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import ClientsList from 'nomad-ui/tests/pages/clients/list';

module('Acceptance | clients list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('/clients should list one page of clients', async function(assert) {
    // Make sure to make more nodes than 1 page to assert that pagination is working
    const nodesCount = 10;
    const pageSize = 8;

    server.createList('node', nodesCount);
    server.createList('agent', 1);

    await ClientsList.visit();

    assert.equal(ClientsList.nodes.length, pageSize);
    assert.ok(ClientsList.hasPagination, 'Pagination found on the page');

    const sortedNodes = server.db.nodes.sortBy('modifyIndex').reverse();

    ClientsList.nodes.forEach((node, index) => {
      assert.equal(node.id, sortedNodes[index].id.split('-')[0], 'Clients are ordered');
    });

    assert.equal(document.title, 'Clients - Nomad');
  });

  test('each client record should show high-level info of the client', async function(assert) {
    const node = server.create('node', 'draining');
    server.createList('agent', 1);

    await ClientsList.visit();

    const nodeRow = ClientsList.nodes.objectAt(0);
    const allocations = server.db.allocations.where({ nodeId: node.id });

    assert.equal(nodeRow.id, node.id.split('-')[0], 'ID');
    assert.equal(nodeRow.name, node.name, 'Name');
    assert.equal(
      nodeRow.compositeStatus.text,
      'draining',
      'Combined status, draining, and eligbility'
    );
    assert.equal(nodeRow.address, node.httpAddr);
    assert.equal(nodeRow.datacenter, node.datacenter, 'Datacenter');
    assert.equal(nodeRow.allocations, allocations.length, '# Allocations');
  });

  test('client status, draining, and eligibility are collapsed into one column that stays sorted', async function(assert) {
    server.createList('agent', 1);

    server.create('node', {
      modifyIndex: 4,
      status: 'ready',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    server.create('node', {
      modifyIndex: 3,
      status: 'initializing',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    server.create('node', {
      modifyIndex: 2,
      status: 'down',
      schedulingEligibility: 'eligible',
      drain: false,
    });
    server.create('node', {
      modifyIndex: 1,
      status: 'ready',
      schedulingEligibility: 'ineligible',
      drain: false,
    });
    server.create('node', 'draining', {
      modifyIndex: 0,
      status: 'ready',
    });

    await ClientsList.visit();

    ClientsList.nodes[0].compositeStatus.as(readyClient => {
      assert.equal(readyClient.text, 'ready');
      assert.ok(readyClient.isUnformatted, 'expected no status class');
      assert.equal(readyClient.tooltip, 'ready / not draining / eligible');
    });

    assert.equal(ClientsList.nodes[1].compositeStatus.text, 'initializing');
    assert.equal(ClientsList.nodes[2].compositeStatus.text, 'down');

    assert.equal(ClientsList.nodes[3].compositeStatus.text, 'ineligible');
    assert.ok(ClientsList.nodes[3].compositeStatus.isWarning, 'expected warning class');

    assert.equal(ClientsList.nodes[4].compositeStatus.text, 'draining');
    assert.ok(ClientsList.nodes[4].compositeStatus.isInfo, 'expected info class');

    await ClientsList.sortBy('compositeStatus');

    assert.deepEqual(ClientsList.nodes.mapBy('compositeStatus.text'), [
      'ready',
      'initializing',
      'ineligible',
      'draining',
      'down',
    ]);

    // Simulate a client state change arriving through polling
    let readyClient = this.owner
      .lookup('service:store')
      .peekAll('node')
      .findBy('modifyIndex', 4);
    readyClient.set('schedulingEligibility', 'ineligible');

    await settled();

    assert.deepEqual(ClientsList.nodes.mapBy('compositeStatus.text'), [
      'initializing',
      'ineligible',
      'ineligible',
      'draining',
      'down',
    ]);
  });

  test('each client should link to the client detail page', async function(assert) {
    server.createList('node', 1);
    server.createList('agent', 1);

    const node = server.db.nodes[0];

    await ClientsList.visit();
    await ClientsList.nodes.objectAt(0).clickRow();

    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('when there are no clients, there is an empty message', async function(assert) {
    server.createList('agent', 1);

    await ClientsList.visit();

    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Clients');
  });

  test('when there are clients, but no matches for a search term, there is an empty message', async function(assert) {
    server.createList('agent', 1);
    server.create('node', { name: 'node' });

    await ClientsList.visit();

    await ClientsList.search('client');
    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Matches');
  });

  test('when accessing clients is forbidden, show a message with a link to the tokens page', async function(assert) {
    server.create('agent');
    server.create('node', { name: 'node' });
    server.pretender.get('/v1/nodes', () => [403, {}, null]);

    await ClientsList.visit();

    assert.equal(ClientsList.error.title, 'Not Authorized');

    await ClientsList.error.seekHelp();

    assert.equal(currentURL(), '/settings/tokens');
  });

  testFacet('Class', {
    facet: ClientsList.facets.class,
    paramName: 'class',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('nodeClass'))).sort();
    },
    async beforeEach() {
      server.create('agent');
      server.createList('node', 2, { nodeClass: 'nc-one' });
      server.createList('node', 2, { nodeClass: 'nc-two' });
      server.createList('node', 2, { nodeClass: 'nc-three' });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.nodeClass),
  });

  testFacet('State', {
    facet: ClientsList.facets.state,
    paramName: 'state',
    expectedOptions: ['Initializing', 'Ready', 'Down', 'Ineligible', 'Draining'],
    async beforeEach() {
      server.create('agent');

      server.createList('node', 2, { status: 'initializing' });
      server.createList('node', 2, { status: 'ready' });
      server.createList('node', 2, { status: 'down' });

      server.createList('node', 2, { schedulingEligibility: 'eligible', drain: false });
      server.createList('node', 2, { schedulingEligibility: 'ineligible', drain: false });
      server.createList('node', 2, { schedulingEligibility: 'ineligible', drain: true });

      await ClientsList.visit();
    },
    filter: (node, selection) => {
      if (selection.includes('draining') && !node.drain) return false;
      if (selection.includes('ineligible') && node.schedulingEligibility === 'eligible')
        return false;

      return selection.includes(node.status);
    },
  });

  testFacet('Datacenters', {
    facet: ClientsList.facets.datacenter,
    paramName: 'dc',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('datacenter'))).sort();
    },
    async beforeEach() {
      server.create('agent');
      server.createList('node', 2, { datacenter: 'pdx-1' });
      server.createList('node', 2, { datacenter: 'nyc-1' });
      server.createList('node', 2, { datacenter: 'ams-1' });
      await ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.datacenter),
  });

  testFacet('Volumes', {
    facet: ClientsList.facets.volume,
    paramName: 'volume',
    expectedOptions(nodes) {
      const flatten = (acc, val) => acc.concat(Object.keys(val));
      return Array.from(new Set(nodes.mapBy('hostVolumes').reduce(flatten, [])));
    },
    async beforeEach() {
      server.create('agent');
      server.createList('node', 2, { hostVolumes: { One: { Name: 'One' } } });
      server.createList('node', 2, { hostVolumes: { One: { Name: 'One' }, Two: { Name: 'Two' } } });
      server.createList('node', 2, { hostVolumes: { Two: { Name: 'Two' } } });
      await ClientsList.visit();
    },
    filter: (node, selection) =>
      Object.keys(node.hostVolumes).find(volume => selection.includes(volume)),
  });

  test('when the facet selections result in no matches, the empty state states why', async function(assert) {
    server.create('agent');
    server.createList('node', 2, { status: 'ready' });

    await ClientsList.visit();

    await ClientsList.facets.state.toggle();
    await ClientsList.facets.state.options.objectAt(0).toggle();
    assert.ok(ClientsList.isEmpty, 'There is an empty message');
    assert.equal(ClientsList.empty.headline, 'No Matches', 'The message is appropriate');
  });

  test('the clients list is immediately filtered based on query params', async function(assert) {
    server.create('agent');
    server.create('node', { nodeClass: 'omg-large' });
    server.create('node', { nodeClass: 'wtf-tiny' });

    await ClientsList.visit({ class: JSON.stringify(['wtf-tiny']) });

    assert.equal(ClientsList.nodes.length, 1, 'Only one client shown due to query param');
  });

  function testFacet(label, { facet, paramName, beforeEach, filter, expectedOptions }) {
    test(`the ${label} facet has the correct options`, async function(assert) {
      await beforeEach();
      await facet.toggle();

      let expectation;
      if (typeof expectedOptions === 'function') {
        expectation = expectedOptions(server.db.nodes);
      } else {
        expectation = expectedOptions;
      }

      assert.deepEqual(
        facet.options.map(option => option.label.trim()),
        expectation,
        'Options for facet are as expected'
      );
    });

    test(`the ${label} facet filters the nodes list by ${label}`, async function(assert) {
      let option;

      await beforeEach();

      await facet.toggle();
      option = facet.options.objectAt(0);
      await option.toggle();

      const selection = [option.key];
      const expectedNodes = server.db.nodes
        .filter(node => filter(node, selection))
        .sortBy('modifyIndex')
        .reverse();

      ClientsList.nodes.forEach((node, index) => {
        assert.equal(
          node.id,
          expectedNodes[index].id.split('-')[0],
          `Node at ${index} is ${expectedNodes[index].id}`
        );
      });
    });

    test(`selecting multiple options in the ${label} facet results in a broader search`, async function(assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const expectedNodes = server.db.nodes
        .filter(node => filter(node, selection))
        .sortBy('modifyIndex')
        .reverse();

      ClientsList.nodes.forEach((node, index) => {
        assert.equal(
          node.id,
          expectedNodes[index].id.split('-')[0],
          `Node at ${index} is ${expectedNodes[index].id}`
        );
      });
    });

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function(assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      assert.equal(
        currentURL(),
        `/clients?${paramName}=${encodeURIComponent(JSON.stringify(selection))}`,
        'URL has the correct query param key and value'
      );
    });
  }
});
