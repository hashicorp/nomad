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
import moment from 'moment';
import { formatBytes, formatHertz } from 'nomad-ui/utils/units';
import PluginDetail from 'nomad-ui/tests/pages/storage/plugins/detail';
import Layout from 'nomad-ui/tests/pages/layout';

module('Acceptance | plugin detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let plugin;

  hooks.beforeEach(function () {
    server.create('node-pool');
    server.create('node');
    plugin = server.create('csi-plugin', { controllerRequired: true });
  });

  test('it passes an accessibility audit', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });
    await a11yAudit(assert);
  });

  test('/csi/plugins/:id should have a breadcrumb trail linking back to Plugins and Storage', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(Layout.breadcrumbFor('csi.index').text, 'Storage');
    assert.equal(Layout.breadcrumbFor('csi.plugins').text, 'Plugins');
    assert.equal(Layout.breadcrumbFor('csi.plugins.plugin').text, plugin.id);
  });

  test('/csi/plugins/:id should show the plugin name in the title', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(document.title, `CSI Plugin ${plugin.id} - Nomad`);
    assert.equal(PluginDetail.title, plugin.id);
  });

  test('/csi/plugins/:id should list additional details for the plugin below the title', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.ok(
      PluginDetail.controllerHealth.includes(
        `${Math.round(
          (plugin.controllersHealthy / plugin.controllersExpected) * 100
        )}%`
      )
    );
    assert.ok(
      PluginDetail.controllerHealth.includes(
        `${plugin.controllersHealthy}/${plugin.controllersExpected}`
      )
    );
    assert.ok(
      PluginDetail.nodeHealth.includes(
        `${Math.round((plugin.nodesHealthy / plugin.nodesExpected) * 100)}%`
      )
    );
    assert.ok(
      PluginDetail.nodeHealth.includes(
        `${plugin.nodesHealthy}/${plugin.nodesExpected}`
      )
    );
    assert.ok(PluginDetail.provider.includes(plugin.provider));
  });

  test('/csi/plugins/:id should list all the controller plugin allocations for the plugin', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(
      PluginDetail.controllerAllocations.length,
      plugin.controllers.length
    );
    plugin.controllers.models
      .sortBy('updateTime')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(
          PluginDetail.controllerAllocations.objectAt(idx).id,
          allocation.allocID
        );
      });
  });

  test('/csi/plugins/:id should list all the node plugin allocations for the plugin', async function (assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(PluginDetail.nodeAllocations.length, plugin.nodes.length);
    plugin.nodes.models
      .sortBy('updateTime')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(
          PluginDetail.nodeAllocations.objectAt(idx).id,
          allocation.allocID
        );
      });
  });

  test('each allocation should have high-level details for the allocation', async function (assert) {
    const controller = plugin.controllers.models
      .sortBy('updateTime')
      .reverse()[0];
    const allocation = server.db.allocations.find(controller.allocID);
    const allocStats = server.db.clientAllocationStats.find(allocation.id);
    const taskGroup = server.db.taskGroups.findBy({
      name: allocation.taskGroup,
      jobId: allocation.jobId,
    });

    const tasks = taskGroup.taskIds.map((id) => server.db.tasks.find(id));
    const cpuUsed = tasks.reduce((sum, task) => sum + task.resources.CPU, 0);
    const memoryUsed = tasks.reduce(
      (sum, task) => sum + task.resources.MemoryMB,
      0
    );

    await PluginDetail.visit({ id: plugin.id });

    PluginDetail.controllerAllocations.objectAt(0).as((allocationRow) => {
      assert.equal(
        allocationRow.shortId,
        allocation.id.split('-')[0],
        'Allocation short ID'
      );
      assert.equal(
        allocationRow.createTime,
        moment(allocation.createTime / 1000000).format('MMM D')
      );
      assert.equal(
        allocationRow.createTooltip,
        moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ')
      );
      assert.equal(
        allocationRow.modifyTime,
        moment(allocation.modifyTime / 1000000).fromNow()
      );
      assert.equal(
        allocationRow.health,
        controller.healthy ? 'Healthy' : 'Unhealthy'
      );
      assert.equal(
        allocationRow.client,
        server.db.nodes.find(allocation.nodeId).id.split('-')[0],
        'Node ID'
      );
      assert.equal(
        allocationRow.clientTooltip.substr(0, 15),
        server.db.nodes.find(allocation.nodeId).name.substr(0, 15),
        'Node Name'
      );
      assert.equal(
        allocationRow.job,
        server.db.jobs.find(allocation.jobId).name,
        'Job name'
      );
      assert.ok(allocationRow.taskGroup, 'Task group name');
      assert.ok(allocationRow.jobVersion, 'Job Version');
      assert.equal(
        allocationRow.cpu,
        Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
        'CPU %'
      );
      const roundedTicks = Math.floor(
        allocStats.resourceUsage.CpuStats.TotalTicks
      );
      assert.equal(
        allocationRow.cpuTooltip,
        `${formatHertz(roundedTicks, 'MHz')} / ${formatHertz(cpuUsed, 'MHz')}`,
        'Detailed CPU information is in a tooltip'
      );
      assert.equal(
        allocationRow.mem,
        allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
        'Memory used'
      );
      assert.equal(
        allocationRow.memTooltip,
        `${formatBytes(
          allocStats.resourceUsage.MemoryStats.RSS
        )} / ${formatBytes(memoryUsed, 'MiB')}`,
        'Detailed memory information is in a tooltip'
      );
    });
  });

  test('each allocation should link to the allocation detail page', async function (assert) {
    const controller = plugin.controllers.models
      .sortBy('updateTime')
      .reverse()[0];

    await PluginDetail.visit({ id: plugin.id });
    await PluginDetail.controllerAllocations.objectAt(0).visit();

    assert.equal(currentURL(), `/allocations/${controller.allocID}`);
  });

  test('when there are no plugin allocations, the tables present empty states', async function (assert) {
    const emptyPlugin = server.create('csi-plugin', {
      controllerRequired: true,
      controllersHealthy: 0,
      controllersExpected: 0,
      nodesHealthy: 0,
      nodesExpected: 0,
    });

    await PluginDetail.visit({ id: emptyPlugin.id });

    assert.ok(PluginDetail.controllerTableIsEmpty);
    assert.equal(
      PluginDetail.controllerEmptyState.headline,
      'No Controller Plugin Allocations'
    );

    assert.ok(PluginDetail.nodeTableIsEmpty);
    assert.equal(
      PluginDetail.nodeEmptyState.headline,
      'No Node Plugin Allocations'
    );
  });

  test('when the plugin is node-only, the controller information is omitted', async function (assert) {
    const nodeOnlyPlugin = server.create('csi-plugin', {
      controllerRequired: false,
    });

    await PluginDetail.visit({ id: nodeOnlyPlugin.id });

    assert.notOk(PluginDetail.controllerAvailabilityIsPresent);
    assert.ok(PluginDetail.nodeAvailabilityIsPresent);

    assert.notOk(PluginDetail.controllerHealthIsPresent);
    assert.notOk(PluginDetail.controllerTableIsPresent);
  });

  test('when there are more than 10 controller or node allocations, only 10 are shown', async function (assert) {
    const manyAllocationsPlugin = server.create('csi-plugin', {
      shallow: true,
      controllerRequired: false,
      nodesExpected: 15,
    });

    await PluginDetail.visit({ id: manyAllocationsPlugin.id });

    assert.equal(PluginDetail.nodeAllocations.length, 10);
  });

  test('the View All links under each allocation table link to a filtered view of the plugins allocation list', async function (assert) {
    const serialize = (arr) => window.encodeURIComponent(JSON.stringify(arr));

    await PluginDetail.visit({ id: plugin.id });
    assert.ok(
      PluginDetail.goToControllerAllocationsText.includes(
        plugin.controllers.models.length
      )
    );
    await PluginDetail.goToControllerAllocations();
    assert.equal(
      currentURL(),
      `/csi/plugins/${plugin.id}/allocations?type=${serialize(['controller'])}`
    );

    await PluginDetail.visit({ id: plugin.id });
    assert.ok(
      PluginDetail.goToNodeAllocationsText.includes(plugin.nodes.models.length)
    );
    await PluginDetail.goToNodeAllocations();
    assert.equal(
      currentURL(),
      `/csi/plugins/${plugin.id}/allocations?type=${serialize(['node'])}`
    );
  });
});
