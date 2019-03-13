import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import setupMirage from 'ember-cli-mirage/test-support/setup-mirage';
import ClientsList from 'nomad-ui/tests/pages/clients/list';

function minimumSetup() {
  server.createList('node', 1);
  server.createList('agent', 1);
}

module('Acceptance | clients list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('/clients should list one page of clients', function(assert) {
    // Make sure to make more nodes than 1 page to assert that pagination is working
    const nodesCount = 10;
    const pageSize = 8;

    server.createList('node', nodesCount);
    server.createList('agent', 1);

    ClientsList.visit();

    assert.equal(ClientsList.nodes.length, pageSize);
    assert.ok(ClientsList.hasPagination, 'Pagination found on the page');

    const sortedNodes = server.db.nodes.sortBy('modifyIndex').reverse();

    ClientsList.nodes.forEach((node, index) => {
      assert.equal(node.id, sortedNodes[index].id.split('-')[0], 'Clients are ordered');
    });
  });

  test('each client record should show high-level info of the client', function(assert) {
    minimumSetup();
    const node = server.db.nodes[0];

    ClientsList.visit();

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

  test('each client should link to the client detail page', function(assert) {
    minimumSetup();
    const node = server.db.nodes[0];

    ClientsList.visit();

    ClientsList.nodes.objectAt(0).clickRow();

    assert.equal(currentURL(), `/clients/${node.id}`);
  });

  test('when there are no clients, there is an empty message', function(assert) {
    server.createList('agent', 1);

    ClientsList.visit();

    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Clients');
  });

  test('when there are clients, but no matches for a search term, there is an empty message', function(assert) {
    server.createList('agent', 1);
    server.create('node', { name: 'node' });

    ClientsList.visit();

    ClientsList.search('client');
    assert.ok(ClientsList.isEmpty);
    assert.equal(ClientsList.empty.headline, 'No Matches');
  });

  test('when accessing clients is forbidden, show a message with a link to the tokens page', function(assert) {
    server.create('agent');
    server.create('node', { name: 'node' });
    server.pretender.get('/v1/nodes', () => [403, {}, null]);

    ClientsList.visit();

    assert.equal(ClientsList.error.title, 'Not Authorized');

    ClientsList.error.seekHelp();

    assert.equal(currentURL(), '/settings/tokens');
  });

  testFacet('Class', {
    facet: ClientsList.facets.class,
    paramName: 'class',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('nodeClass'))).sort();
    },
    beforeEach() {
      server.create('agent');
      server.createList('node', 2, { nodeClass: 'nc-one' });
      server.createList('node', 2, { nodeClass: 'nc-two' });
      server.createList('node', 2, { nodeClass: 'nc-three' });
      ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.nodeClass),
  });

  testFacet('Status', {
    facet: ClientsList.facets.status,
    paramName: 'status',
    expectedOptions: ['Initializing', 'Ready', 'Down'],
    beforeEach() {
      server.create('agent');
      server.createList('node', 2, { status: 'initializing' });
      server.createList('node', 2, { status: 'ready' });
      server.createList('node', 2, { status: 'down' });
      ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.status),
  });

  testFacet('Datacenters', {
    facet: ClientsList.facets.datacenter,
    paramName: 'dc',
    expectedOptions(nodes) {
      return Array.from(new Set(nodes.mapBy('datacenter'))).sort();
    },
    beforeEach() {
      server.create('agent');
      server.createList('node', 2, { datacenter: 'pdx-1' });
      server.createList('node', 2, { datacenter: 'nyc-1' });
      server.createList('node', 2, { datacenter: 'ams-1' });
      ClientsList.visit();
    },
    filter: (node, selection) => selection.includes(node.datacenter),
  });

  testFacet('Flags', {
    facet: ClientsList.facets.flags,
    paramName: 'flags',
    expectedOptions: ['Ineligible', 'Draining'],
    beforeEach() {
      server.create('agent');
      server.createList('node', 2, { schedulingEligibility: 'eligible', drain: false });
      server.createList('node', 2, { schedulingEligibility: 'ineligible', drain: false });
      server.createList('node', 2, { schedulingEligibility: 'ineligible', drain: true });
      ClientsList.visit();
    },
    filter: (node, selection) => {
      if (selection.includes('draining') && !node.drain) return false;
      if (selection.includes('ineligible') && node.schedulingEligibility === 'eligible')
        return false;
      return true;
    },
  });

  test('when the facet selections result in no matches, the empty state states why', function(assert) {
    server.create('agent');
    server.createList('node', 2, { status: 'ready' });

    ClientsList.visit();

    ClientsList.facets.status.toggle();
    ClientsList.facets.status.options.objectAt(0).toggle();
    assert.ok(ClientsList.isEmpty, 'There is an empty message');
    assert.equal(ClientsList.empty.headline, 'No Matches', 'The message is appropriate');
  });

  test('the clients list is immediately filtered based on query params', function(assert) {
    server.create('agent');
    server.create('node', { nodeClass: 'omg-large' });
    server.create('node', { nodeClass: 'wtf-tiny' });

    ClientsList.visit({ class: JSON.stringify(['wtf-tiny']) });

    assert.equal(ClientsList.nodes.length, 1, 'Only one client shown due to query param');
  });

  function testFacet(label, { facet, paramName, beforeEach, filter, expectedOptions }) {
    test(`the ${label} facet has the correct options`, function(assert) {
      beforeEach();

      andThen(() => {
        facet.toggle();
      });

      andThen(() => {
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
    });

    test(`the ${label} facet filters the nodes list by ${label}`, function(assert) {
      let option;

      beforeEach();

      andThen(() => {
        facet.toggle();
      });

      andThen(() => {
        option = facet.options.objectAt(0);
        option.toggle();
      });

      andThen(() => {
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
    });

    test(`selecting multiple options in the ${label} facet results in a broader search`, function(assert) {
      const selection = [];

      beforeEach();

      andThen(() => {
        facet.toggle();
      });

      andThen(() => {
        const option1 = facet.options.objectAt(0);
        const option2 = facet.options.objectAt(1);
        option1.toggle();
        selection.push(option1.key);
        option2.toggle();
        selection.push(option2.key);
      });

      andThen(() => {
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
    });

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, function(assert) {
      const selection = [];

      beforeEach();

      andThen(() => {
        facet.toggle();
      });

      andThen(() => {
        const option1 = facet.options.objectAt(0);
        const option2 = facet.options.objectAt(1);
        option1.toggle();
        selection.push(option1.key);
        option2.toggle();
        selection.push(option2.key);
      });

      andThen(() => {
        assert.equal(
          currentURL(),
          `/clients?${paramName}=${encodeURIComponent(JSON.stringify(selection))}`,
          'URL has the correct query param key and value'
        );
      });
    });
  }
});
