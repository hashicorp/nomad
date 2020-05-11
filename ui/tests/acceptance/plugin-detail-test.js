import { module, test } from 'qunit';
import { currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import moment from 'moment';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import PluginDetail from 'nomad-ui/tests/pages/storage/plugins/detail';

module('Acceptance | plugin detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let plugin;

  hooks.beforeEach(function() {
    server.create('node');
    plugin = server.create('csi-plugin');
  });

  test('/csi/plugins/:id should have a breadcrumb trail linking back to Plugins and Storage', async function(assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(PluginDetail.breadcrumbFor('csi.index').text, 'Storage');
    assert.equal(PluginDetail.breadcrumbFor('csi.plugins').text, 'Plugins');
    assert.equal(PluginDetail.breadcrumbFor('csi.plugins.plugin').text, plugin.id);
  });

  test('/csi/plugins/:id should show the plugin name in the title', async function(assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(document.title, `CSI Plugin ${plugin.id} - Nomad`);
    assert.equal(PluginDetail.title, plugin.id);
  });

  test('/csi/plugins/:id should list additional details for the plugin below the title', async function(assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.ok(
      PluginDetail.controllerHealth.includes(
        `${Math.round((plugin.controllersHealthy / plugin.controllersExpected) * 100)}%`
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
    assert.ok(PluginDetail.nodeHealth.includes(`${plugin.nodesHealthy}/${plugin.nodesExpected}`));
    assert.ok(PluginDetail.provider.includes(plugin.provider));
  });

  test('/csi/plugins/:id should list all the controller plugin allocations for the plugin', async function(assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(PluginDetail.controllerAllocations.length, plugin.controllers.length);
    plugin.controllers.models
      .sortBy('updateTime')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(PluginDetail.controllerAllocations.objectAt(idx).id, allocation.allocID);
      });
  });

  test('/csi/plugins/:id should list all the node plugin allocations for the plugin', async function(assert) {
    await PluginDetail.visit({ id: plugin.id });

    assert.equal(PluginDetail.nodeAllocations.length, plugin.nodes.length);
    plugin.nodes.models
      .sortBy('updateTime')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(PluginDetail.nodeAllocations.objectAt(idx).id, allocation.allocID);
      });
  });

  test('each allocation should have high-level details for the allocation', async function(assert) {
    const controller = plugin.controllers.models.sortBy('updateTime').reverse()[0];
    const allocation = server.db.allocations.find(controller.allocID);
    const allocStats = server.db.clientAllocationStats.find(allocation.id);
    const taskGroup = server.db.taskGroups.findBy({
      name: allocation.taskGroup,
      jobId: allocation.jobId,
    });

    const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));
    const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
    const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

    await PluginDetail.visit({ id: plugin.id });

    PluginDetail.controllerAllocations.objectAt(0).as(allocationRow => {
      assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'Allocation short ID');
      assert.equal(
        allocationRow.createTime,
        moment(allocation.createTime / 1000000).format('MMM D')
      );
      assert.equal(
        allocationRow.createTooltip,
        moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ')
      );
      assert.equal(allocationRow.modifyTime, moment(allocation.modifyTime / 1000000).fromNow());
      assert.equal(allocationRow.health, controller.healthy ? 'Healthy' : 'Unhealthy');
      assert.equal(
        allocationRow.client,
        server.db.nodes.find(allocation.nodeId).id.split('-')[0],
        'Node ID'
      );
      assert.equal(allocationRow.job, server.db.jobs.find(allocation.jobId).name, 'Job name');
      assert.ok(allocationRow.taskGroup, 'Task group name');
      assert.ok(allocationRow.jobVersion, 'Job Version');
      assert.equal(
        allocationRow.cpu,
        Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
        'CPU %'
      );
      assert.equal(
        allocationRow.cpuTooltip,
        `${Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks)} / ${cpuUsed} MHz`,
        'Detailed CPU information is in a tooltip'
      );
      assert.equal(
        allocationRow.mem,
        allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
        'Memory used'
      );
      assert.equal(
        allocationRow.memTooltip,
        `${formatBytes([allocStats.resourceUsage.MemoryStats.RSS])} / ${memoryUsed} MiB`,
        'Detailed memory information is in a tooltip'
      );
    });
  });

  test('each allocation should link to the allocation detail page', async function(assert) {
    const controller = plugin.controllers.models.sortBy('updateTime').reverse()[0];

    await PluginDetail.visit({ id: plugin.id });
    await PluginDetail.controllerAllocations.objectAt(0).visit();

    assert.equal(currentURL(), `/allocations/${controller.allocID}`);
  });

  test('when there are no plugin allocations, the tables present empty states', async function(assert) {
    const emptyPlugin = server.create('csi-plugin', {
      controllersHealthy: 0,
      controllersExpected: 0,
      nodesHealthy: 0,
      nodesExpected: 0,
    });

    await PluginDetail.visit({ id: emptyPlugin.id });

    assert.ok(PluginDetail.controllerTableIsEmpty);
    assert.equal(PluginDetail.controllerEmptyState.headline, 'No Controller Plugin Allocations');

    assert.ok(PluginDetail.nodeTableIsEmpty);
    assert.equal(PluginDetail.nodeEmptyState.headline, 'No Node Plugin Allocations');
  });
});
