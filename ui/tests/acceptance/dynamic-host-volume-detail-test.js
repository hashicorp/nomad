/**
 * Copyright IBM Corp. 2015, 2025
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
import VolumeDetail from 'nomad-ui/tests/pages/storage/dynamic-host-volumes/detail';
import Layout from 'nomad-ui/tests/pages/layout';
import percySnapshot from '@percy/ember';

const assignAlloc = (volume, alloc) => {
  volume.allocations.add(alloc);
  volume.save();
};

module('Acceptance | dynamic host volume detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let volume;

  hooks.beforeEach(function () {
    server.create('node-pool');
    server.create('node');
    server.create('job', {
      name: 'dhv-job',
    });
    volume = server.create('dynamic-host-volume', {
      nodeId: server.db.nodes[0].id,
    });
  });

  test('it passes an accessibility audit', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await a11yAudit(assert);
  });

  test('/storage/volumes/:id should have a breadcrumb trail linking back to Volumes and Storage', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.equal(Layout.breadcrumbFor('storage.index').text, 'Storage');
    assert.equal(
      Layout.breadcrumbFor('storage.volumes.dynamic-host-volume').text,
      volume.name
    );
  });

  test('/storage/volumes/:id should show the volume name in the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.equal(document.title, `Dynamic Host Volume ${volume.name} - Nomad`);
    assert.equal(VolumeDetail.title, volume.name);
  });

  test('/storage/volumes/:id should list additional details for the volume below the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    assert.ok(VolumeDetail.node.includes(volume.node.name));
    assert.ok(VolumeDetail.plugin.includes(volume.pluginID));
    assert.notOk(
      VolumeDetail.hasNamespace,
      'Namespace is omitted when there is only one namespace'
    );
    assert.equal(VolumeDetail.capacity, 'Capacity 9.54 MiB');
  });

  test('/storage/volumes/:id should list all allocations the volume is attached to', async function (assert) {
    const allocations = server.createList('allocation', 3);
    allocations.forEach((alloc) => assignAlloc(volume, alloc));

    // Freeze moment's time reference so relative times ("2 days ago") are
    // deterministic across Percy snapshot runs.
    const originalMomentNow = moment.now;
    const latestModifyTime = Math.max(...allocations.map((a) => a.modifyTime));
    // Pin "now" to 1 hour after the latest allocation modify time
    moment.now = () => Math.floor(latestModifyTime / 1000000) + 3600000;

    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.equal(VolumeDetail.allocations.length, allocations.length);
    allocations
      .sortBy('modifyIndex')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(allocation.id, VolumeDetail.allocations.objectAt(idx).id);
      });
    await percySnapshot(assert);

    moment.now = originalMomentNow;
  });

  test('each allocation should have high-level details for the allocation', async function (assert) {
    const allocation = server.create('allocation', { clientStatus: 'running' });
    assignAlloc(volume, allocation);

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

    await VolumeDetail.visit({ id: `${volume.id}@default` });
    VolumeDetail.allocations.objectAt(0).as((allocationRow) => {
      assert.equal(
        allocationRow.shortId,
        allocation.id.split('-')[0],
        'Allocation short ID'
      );
      assert.equal(
        allocationRow.createTime,
        moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ'),
        'Allocation create time'
      );
      assert.equal(
        allocationRow.modifyTime,
        moment(allocation.modifyTime / 1000000).fromNow(),
        'Allocation modify time'
      );
      assert.equal(
        allocationRow.status,
        allocation.clientStatus,
        'Client status'
      );
      assert.equal(
        allocationRow.job,
        server.db.jobs.find(allocation.jobId).name,
        'Job name'
      );
      assert.ok(allocationRow.taskGroup, 'Task group name');
      assert.ok(allocationRow.jobVersion, 'Job Version');
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
    const allocation = server.create('allocation');
    assignAlloc(volume, allocation);

    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await VolumeDetail.allocations.objectAt(0).visit();

    assert.equal(currentURL(), `/allocations/${allocation.id}`);
  });

  test('when there are no allocations, the table presents an empty state', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(VolumeDetail.allocationsTableIsEmpty);
    assert.equal(VolumeDetail.allocationsEmptyState.headline, 'No Allocations');
  });

  test('Capabilities table shows access mode and attachment mode', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    assert.equal(
      VolumeDetail.capabilities.objectAt(0).accessMode,
      'single-node-writer'
    );
    assert.equal(
      VolumeDetail.capabilities.objectAt(0).attachmentMode,
      'file-system'
    );
    assert.equal(
      VolumeDetail.capabilities.objectAt(1).accessMode,
      'single-node-reader-only'
    );
    assert.equal(
      VolumeDetail.capabilities.objectAt(1).attachmentMode,
      'block-device'
    );
  });
});

// Namespace test: details shows the namespace
module(
  'Acceptance | dynamic volume detail (with namespaces)',
  function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);

    let volume;

    hooks.beforeEach(function () {
      server.createList('namespace', 2);
      server.create('node-pool');
      server.create('node');
      volume = server.create('dynamic-host-volume');
    });

    test('/storage/volumes/:id detail ribbon includes the namespace of the volume', async function (assert) {
      await VolumeDetail.visit({ id: `${volume.id}@${volume.namespaceId}` });

      assert.ok(VolumeDetail.hasNamespace);
      assert.ok(
        VolumeDetail.namespace.includes(volume.namespaceId || 'default')
      );
    });
  }
);
