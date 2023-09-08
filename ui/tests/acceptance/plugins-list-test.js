/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import PluginsList from 'nomad-ui/tests/pages/storage/plugins/list';

module('Acceptance | plugins list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    server.create('node-pool');
    server.create('node');
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function (assert) {
    await PluginsList.visit();
    await a11yAudit(assert);
  });

  test('visiting /csi/plugins', async function (assert) {
    await PluginsList.visit();

    assert.equal(currentURL(), '/csi/plugins');
    assert.equal(document.title, 'CSI Plugins - Nomad');
  });

  test('/csi/plugins should list the first page of plugins sorted by id', async function (assert) {
    const pluginCount = PluginsList.pageSize + 1;
    server.createList('csi-plugin', pluginCount, { shallow: true });

    await PluginsList.visit();

    const sortedPlugins = server.db.csiPlugins.sortBy('id');
    assert.equal(PluginsList.plugins.length, PluginsList.pageSize);
    PluginsList.plugins.forEach((plugin, index) => {
      assert.equal(plugin.id, sortedPlugins[index].id, 'Plugins are ordered');
    });
  });

  test('each plugin row should contain information about the plugin', async function (assert) {
    const plugin = server.create('csi-plugin', {
      shallow: true,
      controllerRequired: true,
    });

    await PluginsList.visit();

    const pluginRow = PluginsList.plugins.objectAt(0);
    const controllerHealthStr =
      plugin.controllersHealthy > 0 ? 'Healthy' : 'Unhealthy';
    const nodeHealthStr = plugin.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.equal(pluginRow.id, plugin.id);
    assert.equal(
      pluginRow.controllerHealth,
      `${controllerHealthStr} (${plugin.controllersHealthy}/${plugin.controllersExpected})`
    );
    assert.equal(
      pluginRow.nodeHealth,
      `${nodeHealthStr} (${plugin.nodesHealthy}/${plugin.nodesExpected})`
    );
    assert.equal(pluginRow.provider, plugin.provider);
  });

  test('node only plugins explain that there is no controller health for this plugin type', async function (assert) {
    const plugin = server.create('csi-plugin', {
      shallow: true,
      controllerRequired: false,
    });

    await PluginsList.visit();

    const pluginRow = PluginsList.plugins.objectAt(0);
    const nodeHealthStr = plugin.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.equal(pluginRow.id, plugin.id);
    assert.equal(pluginRow.controllerHealth, 'Node Only');
    assert.equal(
      pluginRow.nodeHealth,
      `${nodeHealthStr} (${plugin.nodesHealthy}/${plugin.nodesExpected})`
    );
    assert.equal(pluginRow.provider, plugin.provider);
  });

  test('each plugin row should link to the corresponding plugin', async function (assert) {
    const plugin = server.create('csi-plugin', { shallow: true });

    await PluginsList.visit();

    await PluginsList.plugins.objectAt(0).clickName();
    assert.equal(currentURL(), `/csi/plugins/${plugin.id}`);

    await PluginsList.visit();
    assert.equal(currentURL(), '/csi/plugins');

    await PluginsList.plugins.objectAt(0).clickRow();
    assert.equal(currentURL(), `/csi/plugins/${plugin.id}`);
  });

  test('when there are no plugins, there is an empty message', async function (assert) {
    await PluginsList.visit();

    assert.ok(PluginsList.isEmpty);
    assert.equal(PluginsList.emptyState.headline, 'No Plugins');
  });

  test('when there are plugins, but no matches for a search, there is an empty message', async function (assert) {
    server.create('csi-plugin', { id: 'cat 1', shallow: true });
    server.create('csi-plugin', { id: 'cat 2', shallow: true });

    await PluginsList.visit();

    await PluginsList.search('dog');
    assert.ok(PluginsList.isEmpty);
    assert.equal(PluginsList.emptyState.headline, 'No Matches');
  });

  test('search resets the current page', async function (assert) {
    server.createList('csi-plugin', PluginsList.pageSize + 1, {
      shallow: true,
    });

    await PluginsList.visit();
    await PluginsList.nextPage();

    assert.equal(currentURL(), '/csi/plugins?page=2');

    await PluginsList.search('foobar');

    assert.equal(currentURL(), '/csi/plugins?search=foobar');
  });

  test('when accessing plugins is forbidden, a message is shown with a link to the tokens page', async function (assert) {
    server.pretender.get('/v1/plugins', () => [403, {}, null]);

    await PluginsList.visit();
    assert.equal(PluginsList.error.title, 'Not Authorized');

    await PluginsList.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });

  pageSizeSelect({
    resourceName: 'plugin',
    pageObject: PluginsList,
    pageObjectList: PluginsList.plugins,
    async setup() {
      server.createList('csi-plugin', PluginsList.pageSize, { shallow: true });
      await PluginsList.visit();
    },
  });
});
