/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { getPageTitle } from 'ember-page-title/test-support';
import { currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import setupAuthenticatedAcceptance from 'nomad-ui/tests/helpers/setup-authenticated-acceptance';
import moment from 'moment';
import { formatBytes, formatHertz } from 'nomad-ui/utils/units';
import VolumeDetail from 'nomad-ui/tests/pages/storage/volumes/detail';
import Layout from 'nomad-ui/tests/pages/layout';

const assignWriteAlloc = (volume, alloc) => {
  volume.writeAllocs.add(alloc);
  volume.allocations.add(alloc);
  volume.save();
};

const assignReadAlloc = (volume, alloc) => {
  volume.readAllocs.add(alloc);
  volume.allocations.add(alloc);
  volume.save();
};

module('Acceptance | volume detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupAuthenticatedAcceptance(hooks);

  let volume;

  hooks.beforeEach(function () {
    this.server.create('node-pool');
    this.server.create('node');
    this.server.create('csi-plugin', { createVolumes: false });
    volume = this.server.create('csi-volume');
  });

  test('it passes an accessibility audit', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await a11yAudit(assert);
  });

  test('/storage/volumes/:id should have a breadcrumb trail linking back to Volumes and Storage', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.deepEqual(Layout.breadcrumbFor('storage.index').text, 'Storage');
    assert.deepEqual(
      Layout.breadcrumbFor('storage.volumes.volume').text,
      volume.name,
    );
  });

  test('/storage/volumes/:id should show the volume name in the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    const pageTitle = getPageTitle();
    assert.ok(pageTitle.startsWith(`CSI Volume ${volume.name}`));
    assert.ok(pageTitle.endsWith(' - Nomad'));
    assert.deepEqual(VolumeDetail.title, volume.name);
  });

  test('/storage/volumes/:id should list additional details for the volume below the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(
      VolumeDetail.health.includes(
        volume.schedulable ? 'Schedulable' : 'Unschedulable',
      ),
    );
    assert.ok(VolumeDetail.provider.includes(volume.provider));
    assert.ok(VolumeDetail.externalId.includes(volume.externalId));
    assert.notOk(
      VolumeDetail.hasNamespace,
      'Namespace is omitted when there is only one namespace',
    );
  });

  test('/storage/volumes/:id should list all write allocations the volume is attached to', async function (assert) {
    const writeAllocations = this.server.createList('allocation', 2);
    const readAllocations = this.server.createList('allocation', 3);
    writeAllocations.forEach((alloc) => assignWriteAlloc(volume, alloc));
    readAllocations.forEach((alloc) => assignReadAlloc(volume, alloc));

    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.deepEqual(
      VolumeDetail.writeAllocations.length,
      writeAllocations.length,
    );
    writeAllocations
      .sortBy('modifyIndex')
      .reverse()
      .forEach((allocation, idx) => {
        assert.deepEqual(
          allocation.id,
          VolumeDetail.writeAllocations.objectAt(idx).id,
        );
      });
  });

  test('/storage/volumes/:id should list all read allocations the volume is attached to', async function (assert) {
    const writeAllocations = this.server.createList('allocation', 2);
    const readAllocations = this.server.createList('allocation', 3);
    writeAllocations.forEach((alloc) => assignWriteAlloc(volume, alloc));
    readAllocations.forEach((alloc) => assignReadAlloc(volume, alloc));

    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.deepEqual(
      VolumeDetail.readAllocations.length,
      readAllocations.length,
    );
    readAllocations
      .sortBy('modifyIndex')
      .reverse()
      .forEach((allocation, idx) => {
        assert.deepEqual(
          allocation.id,
          VolumeDetail.readAllocations.objectAt(idx).id,
        );
      });
  });

  test('each allocation should have high-level details for the allocation', async function (assert) {
    const allocation = this.server.create('allocation', {
      clientStatus: 'running',
    });
    assignWriteAlloc(volume, allocation);

    const allocStats = this.server.db.clientAllocationStats.find(allocation.id);
    const taskGroup = this.server.db.taskGroups.findBy({
      name: allocation.taskGroup,
      jobId: allocation.jobId,
    });

    const tasks = taskGroup.taskIds.map((id) => this.server.db.tasks.find(id));
    const cpuUsed = tasks.reduce((sum, task) => sum + task.resources.CPU, 0);
    const memoryUsed = tasks.reduce(
      (sum, task) => sum + task.resources.MemoryMB,
      0,
    );

    await VolumeDetail.visit({ id: `${volume.id}@default` });

    VolumeDetail.writeAllocations.objectAt(0).as((allocationRow) => {
      assert.deepEqual(
        allocationRow.shortId,
        allocation.id.split('-')[0],
        'Allocation short ID',
      );
      assert.deepEqual(
        allocationRow.createTime,
        moment(allocation.createTime / 1000000).format('MMM DD HH:mm:ss ZZ'),
        'Allocation create time',
      );
      assert.deepEqual(
        allocationRow.modifyTime,
        moment(allocation.modifyTime / 1000000).fromNow(),
        'Allocation modify time',
      );
      assert.deepEqual(
        allocationRow.status,
        allocation.clientStatus,
        'Client status',
      );
      assert.deepEqual(
        allocationRow.job,
        this.server.db.jobs.find(allocation.jobId).name,
        'Job name',
      );
      assert.ok(allocationRow.taskGroup, 'Task group name');
      assert.ok(allocationRow.jobVersion, 'Job Version');
      assert.deepEqual(
        allocationRow.client,
        this.server.db.nodes.find(allocation.nodeId).id.split('-')[0],
        'Node ID',
      );
      assert.deepEqual(
        allocationRow.clientTooltip.substr(0, 15),
        this.server.db.nodes.find(allocation.nodeId).name.substr(0, 15),
        'Node Name',
      );
      assert.strictEqual(
        Number(allocationRow.cpu),
        Math.floor(allocStats.resourceUsage.CpuStats.TotalTicks) / cpuUsed,
        'CPU %',
      );
      const roundedTicks = Math.floor(
        allocStats.resourceUsage.CpuStats.TotalTicks,
      );
      assert.deepEqual(
        allocationRow.cpuTooltip,
        `${formatHertz(roundedTicks, 'MHz')} / ${formatHertz(cpuUsed, 'MHz')}`,
        'Detailed CPU information is in a tooltip',
      );
      assert.strictEqual(
        Number(allocationRow.mem),
        allocStats.resourceUsage.MemoryStats.RSS / 1024 / 1024 / memoryUsed,
        'Memory used',
      );
      assert.deepEqual(
        allocationRow.memTooltip,
        `${formatBytes(
          allocStats.resourceUsage.MemoryStats.RSS,
        )} / ${formatBytes(memoryUsed, 'MiB')}`,
        'Detailed memory information is in a tooltip',
      );
    });
  });

  test('each allocation should link to the allocation detail page', async function (assert) {
    const allocation = this.server.create('allocation');
    assignWriteAlloc(volume, allocation);

    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await VolumeDetail.writeAllocations.objectAt(0).visit();

    assert.deepEqual(currentURL(), `/allocations/${allocation.id}`);
  });

  test('when there are no write allocations, the table presents an empty state', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(VolumeDetail.writeTableIsEmpty);
    assert.deepEqual(
      VolumeDetail.writeEmptyState.headline,
      'No Write Allocations',
    );
  });

  test('when there are no read allocations, the table presents an empty state', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(VolumeDetail.readTableIsEmpty);
    assert.deepEqual(
      VolumeDetail.readEmptyState.headline,
      'No Read Allocations',
    );
  });

  test('the constraints table shows access mode and attachment mode', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.deepEqual(VolumeDetail.constraints.accessMode, volume.accessMode);
    assert.deepEqual(
      VolumeDetail.constraints.attachmentMode,
      volume.attachmentMode,
    );
  });
});

// Namespace test: details shows the namespace
module('Acceptance | volume detail (with namespaces)', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let volume;

  hooks.beforeEach(function () {
    this.server.createList('namespace', 2);
    this.server.create('node-pool');
    this.server.create('node');
    this.server.create('csi-plugin', { createVolumes: false });
    volume = this.server.create('csi-volume');
  });

  test('/storage/volumes/:id detail ribbon includes the namespace of the volume', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@${volume.namespaceId}` });

    assert.ok(VolumeDetail.hasNamespace);
    assert.ok(VolumeDetail.namespace.includes(volume.namespaceId || 'default'));
  });
});
