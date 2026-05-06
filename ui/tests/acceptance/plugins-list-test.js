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
import setupAuthenticatedAcceptance from 'nomad-ui/tests/helpers/setup-authenticated-acceptance';
import pageSizeSelect from './behaviors/page-size-select';
import PluginsList from 'nomad-ui/tests/pages/storage/plugins/list';

module('Acceptance | plugins list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupAuthenticatedAcceptance(hooks);

  hooks.beforeEach(function () {
    this.server.create('node-pool');
    this.server.create('node');
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function (assert) {
    await PluginsList.visit();
    await a11yAudit(assert);
  });

  test('visiting /storage/plugins', async function (assert) {
    await PluginsList.visit();

    assert.deepEqual(currentURL(), '/storage/plugins');
    const pageTitle = getPageTitle();
    assert.ok(pageTitle.startsWith('CSI Plugins'));
    assert.ok(pageTitle.endsWith(' - Nomad'));
  });

  test('/storage/plugins should list the first page of plugins sorted by id', async function (assert) {
    const pluginCount = PluginsList.pageSize + 1;
    this.server.createList('csi-plugin', pluginCount, { shallow: true });

    await PluginsList.visit();

    const sortedPlugins = this.server.db.csiPlugins.sortBy('id');
    assert.deepEqual(PluginsList.plugins.length, PluginsList.pageSize);
    PluginsList.plugins.forEach((plugin, index) => {
      assert.deepEqual(
        plugin.id,
        sortedPlugins[index].id,
        'Plugins are ordered',
      );
    });
  });

  test('each plugin row should contain information about the plugin', async function (assert) {
    const plugin = this.server.create('csi-plugin', {
      shallow: true,
      controllerRequired: true,
    });

    await PluginsList.visit();

    const pluginRow = PluginsList.plugins.objectAt(0);
    const controllerHealthStr =
      plugin.controllersHealthy > 0 ? 'Healthy' : 'Unhealthy';
    const nodeHealthStr = plugin.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.deepEqual(pluginRow.id, plugin.id);
    assert.deepEqual(
      pluginRow.controllerHealth,
      `${controllerHealthStr} (${plugin.controllersHealthy}/${plugin.controllersExpected})`,
    );
    assert.deepEqual(
      pluginRow.nodeHealth,
      `${nodeHealthStr} (${plugin.nodesHealthy}/${plugin.nodesExpected})`,
    );
    assert.deepEqual(pluginRow.provider, plugin.provider);
  });

  test('node only plugins explain that there is no controller health for this plugin type', async function (assert) {
    const plugin = this.server.create('csi-plugin', {
      shallow: true,
      controllerRequired: false,
    });

    await PluginsList.visit();

    const pluginRow = PluginsList.plugins.objectAt(0);
    const nodeHealthStr = plugin.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.deepEqual(pluginRow.id, plugin.id);
    assert.deepEqual(pluginRow.controllerHealth, 'Node Only');
    assert.deepEqual(
      pluginRow.nodeHealth,
      `${nodeHealthStr} (${plugin.nodesHealthy}/${plugin.nodesExpected})`,
    );
    assert.deepEqual(pluginRow.provider, plugin.provider);
  });

  test('each plugin row should link to the corresponding plugin', async function (assert) {
    const plugin = this.server.create('csi-plugin', { shallow: true });

    await PluginsList.visit();

    await PluginsList.plugins.objectAt(0).clickName();
    assert.deepEqual(currentURL(), `/storage/plugins/${plugin.id}`);

    await PluginsList.visit();
    assert.deepEqual(currentURL(), '/storage/plugins');

    await PluginsList.plugins.objectAt(0).clickRow();
    assert.deepEqual(currentURL(), `/storage/plugins/${plugin.id}`);
  });

  test('when there are no plugins, there is an empty message', async function (assert) {
    await PluginsList.visit();

    assert.ok(PluginsList.isEmpty);
    assert.deepEqual(PluginsList.emptyState.headline, 'No Plugins');
  });

  test('when there are plugins, but no matches for a search, there is an empty message', async function (assert) {
    this.server.create('csi-plugin', { id: 'cat 1', shallow: true });
    this.server.create('csi-plugin', { id: 'cat 2', shallow: true });

    await PluginsList.visit();

    await PluginsList.search('dog');
    assert.ok(PluginsList.isEmpty);
    assert.deepEqual(PluginsList.emptyState.headline, 'No Matches');
  });

  test('search resets the current page', async function (assert) {
    this.server.createList('csi-plugin', PluginsList.pageSize + 1, {
      shallow: true,
    });

    await PluginsList.visit();
    await PluginsList.nextPage();

    assert.deepEqual(currentURL(), '/storage/plugins?page=2');

    await PluginsList.search('foobar');

    assert.deepEqual(currentURL(), '/storage/plugins?search=foobar');
  });

  test('when accessing plugins is forbidden, a message is shown with a link to the tokens page', async function (assert) {
    this.server.pretender.get('/v1/plugins', () => [403, {}, null]);

    await PluginsList.visit();
    assert.deepEqual(PluginsList.error.title, 'Not Authorized');

    await PluginsList.error.seekHelp();
    assert.deepEqual(currentURL(), '/settings/tokens');
  });

  pageSizeSelect({
    resourceName: 'plugin',
    pageObject: PluginsList,
    pageObjectList: PluginsList.plugins,
    async setup() {
      this.server.createList('csi-plugin', PluginsList.pageSize, {
        shallow: true,
      });
      await PluginsList.visit();
    },
  });
});
