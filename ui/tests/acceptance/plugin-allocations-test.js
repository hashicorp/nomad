/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { module, test } from 'qunit';
import { currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import PluginAllocations from 'nomad-ui/tests/pages/storage/plugins/plugin/allocations';

module('Acceptance | plugin allocations', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let plugin;

  hooks.beforeEach(function () {
    server.create('node-pool');
    server.create('node');
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function (assert) {
    plugin = server.create('csi-plugin', {
      shallow: true,
      controllerRequired: true,
      controllersExpected: 3,
      nodesExpected: 3,
    });

    await PluginAllocations.visit({ id: plugin.id });
    await a11yAudit(assert);
  });

  test('/csi/plugins/:id/allocations shows all allocations in a single table', async function (assert) {
    plugin = server.create('csi-plugin', {
      shallow: true,
      controllerRequired: true,
      controllersExpected: 3,
      nodesExpected: 3,
    });

    await PluginAllocations.visit({ id: plugin.id });
    assert.equal(PluginAllocations.allocations.length, 6);
  });

  pageSizeSelect({
    resourceName: 'allocation',
    pageObject: PluginAllocations,
    pageObjectList: PluginAllocations.allocations,
    async setup() {
      const total = PluginAllocations.pageSize;
      plugin = server.create('csi-plugin', {
        shallow: true,
        controllerRequired: true,
        controllersExpected: Math.floor(total / 2),
        nodesExpected: Math.ceil(total / 2),
      });

      await PluginAllocations.visit({ id: plugin.id });
    },
  });

  testFacet('Health', {
    facet: PluginAllocations.facets.health,
    paramName: 'healthy',
    async beforeEach() {
      plugin = server.create('csi-plugin', {
        shallow: true,
        controllerRequired: true,
        controllersExpected: 3,
        nodesExpected: 3,
      });

      await PluginAllocations.visit({ id: plugin.id });
    },
    filter: (allocation, selection) =>
      selection.includes(allocation.healthy.toString()),
  });

  testFacet('Type', {
    facet: PluginAllocations.facets.type,
    paramName: 'type',
    async beforeEach() {
      plugin = server.create('csi-plugin', {
        shallow: true,
        controllerRequired: true,
        controllersExpected: 3,
        nodesExpected: 3,
      });

      await PluginAllocations.visit({ id: plugin.id });
    },
    filter: (allocation, selection) => {
      if (selection.length === 0 || selection.length === 2) return true;
      if (selection[0] === 'controller')
        return plugin.controllers.models.includes(allocation);
      return plugin.nodes.models.includes(allocation);
    },
  });

  function testFacet(label, { facet, paramName, beforeEach, filter }) {
    test(`the ${label} facet filters the allocations list by ${label}`, async function (assert) {
      let option;

      await beforeEach();
      await facet.toggle();

      option = facet.options.objectAt(0);
      await option.toggle();

      const selection = [option.key];
      const allAllocations = [
        ...plugin.controllers.models,
        ...plugin.nodes.models,
      ];
      const expectedAllocations = allAllocations
        .filter((allocation) => filter(allocation, selection))
        .sortBy('updateTime');

      PluginAllocations.allocations.forEach((allocation, index) => {
        assert.equal(allocation.id, expectedAllocations[index].allocID);
      });
    });

    test(`selecting multiple options in the ${label} facet results in a broader search`, async function (assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const allAllocations = [
        ...plugin.controllers.models,
        ...plugin.nodes.models,
      ];
      const expectedAllocations = allAllocations
        .filter((allocation) => filter(allocation, selection))
        .sortBy('updateTime');

      PluginAllocations.allocations.forEach((allocation, index) => {
        assert.equal(allocation.id, expectedAllocations[index].allocID);
      });
    });

    test(`selecting options in the ${label} facet updates the ${paramName} query param`, async function (assert) {
      const selection = [];

      await beforeEach();
      await facet.toggle();

      const option1 = facet.options.objectAt(0);
      const option2 = facet.options.objectAt(1);
      await option1.toggle();
      selection.push(option1.key);
      await option2.toggle();
      selection.push(option2.key);

      const queryString = `${paramName}=${window.encodeURIComponent(
        JSON.stringify(selection)
      )}`;

      assert.equal(
        currentURL(),
        `/csi/plugins/${plugin.id}/allocations?${queryString}`
      );
    });
  }
});
