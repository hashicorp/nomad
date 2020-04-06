import { module, test } from 'qunit';
import { currentURL } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import moment from 'moment';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';
import VolumeDetail from 'nomad-ui/tests/pages/storage/volumes/detail';

const assignWriteAlloc = (volume, alloc) => {
  volume.writeAllocs.add(alloc);
  volume.save();
};

const assignReadAlloc = (volume, alloc) => {
  volume.readAllocs.add(alloc);
  volume.save();
};

module('Acceptance | volume detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let volume;

  hooks.beforeEach(function() {
    server.create('node');
    server.create('csi-plugin', { createVolumes: false });
    volume = server.create('csi-volume');
  });

  test('/csi/volumes/:id should have a breadcrumb trail linking back to Volumes and CSI', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.equal(VolumeDetail.breadcrumbFor('csi.index').text, 'Storage');
    assert.equal(VolumeDetail.breadcrumbFor('csi.volumes').text, 'Volumes');
    assert.equal(VolumeDetail.breadcrumbFor('csi.volumes.volume').text, volume.name);
  });

  test('/csi/volumes/:id should show the volume name in the title', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.equal(document.title, `CSI Volume ${volume.name} - Nomad`);
    assert.equal(VolumeDetail.title, volume.name);
  });

  test('/csi/volumes/:id should list additional details for the volume below the title', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.ok(VolumeDetail.health.includes(volume.schedulable ? 'Schedulable' : 'Unschedulable'));
    assert.ok(VolumeDetail.provider.includes(volume.provider));
    assert.ok(VolumeDetail.externalId.includes(volume.externalId));
    assert.notOk(
      VolumeDetail.hasNamespace,
      'Namespace is omitted when there is only one namespace'
    );
  });

  test('/csi/volumes/:id should list all write allocations the volume is attached to', async function(assert) {
    const writeAllocations = server.createList('allocation', 2);
    const readAllocations = server.createList('allocation', 3);
    writeAllocations.forEach(alloc => assignWriteAlloc(volume, alloc));
    readAllocations.forEach(alloc => assignReadAlloc(volume, alloc));

    await VolumeDetail.visit({ id: volume.id });

    assert.equal(VolumeDetail.writeAllocations.length, writeAllocations.length);
    writeAllocations
      .sortBy('modifyIndex')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(allocation.id, VolumeDetail.writeAllocations.objectAt(idx).id);
      });
  });

  test('/csi/volumes/:id should list all read allocations the volume is attached to', async function(assert) {
    const writeAllocations = server.createList('allocation', 2);
    const readAllocations = server.createList('allocation', 3);
    writeAllocations.forEach(alloc => assignWriteAlloc(volume, alloc));
    readAllocations.forEach(alloc => assignReadAlloc(volume, alloc));

    await VolumeDetail.visit({ id: volume.id });

    assert.equal(VolumeDetail.readAllocations.length, readAllocations.length);
    readAllocations
      .sortBy('modifyIndex')
      .reverse()
      .forEach((allocation, idx) => {
        assert.equal(allocation.id, VolumeDetail.readAllocations.objectAt(idx).id);
      });
  });

  test('each allocation should have high-level details for the allocation', async function(assert) {
    const allocation = server.create('allocation', { clientStatus: 'running' });
    assignWriteAlloc(volume, allocation);

    const allocStats = server.db.clientAllocationStats.find(allocation.id);
    const taskGroup = server.db.taskGroups.findBy({
      name: allocation.taskGroup,
      jobId: allocation.jobId,
    });

    const tasks = taskGroup.taskIds.map(id => server.db.tasks.find(id));
    const cpuUsed = tasks.reduce((sum, task) => sum + task.Resources.CPU, 0);
    const memoryUsed = tasks.reduce((sum, task) => sum + task.Resources.MemoryMB, 0);

    await VolumeDetail.visit({ id: volume.id });

    VolumeDetail.writeAllocations.objectAt(0).as(allocationRow => {
      assert.equal(allocationRow.shortId, allocation.id.split('-')[0], 'Allocation short ID');
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
      assert.equal(allocationRow.status, allocation.clientStatus, 'Client status');
      assert.equal(allocationRow.job, server.db.jobs.find(allocation.jobId).name, 'Job name');
      assert.ok(allocationRow.taskGroup, 'Task group name');
      assert.ok(allocationRow.jobVersion, 'Job Version');
      assert.equal(
        allocationRow.client,
        server.db.nodes.find(allocation.nodeId).id.split('-')[0],
        'Node ID'
      );
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
    const allocation = server.create('allocation');
    assignWriteAlloc(volume, allocation);

    await VolumeDetail.visit({ id: volume.id });
    await VolumeDetail.writeAllocations.objectAt(0).visit();

    assert.equal(currentURL(), `/allocations/${allocation.id}`);
  });

  test('when there are no write allocations, the table presents an empty state', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.ok(VolumeDetail.writeTableIsEmpty);
    assert.equal(VolumeDetail.writeEmptyState.headline, 'No Write Allocations');
  });

  test('when there are no read allocations, the table presents an empty state', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.ok(VolumeDetail.readTableIsEmpty);
    assert.equal(VolumeDetail.readEmptyState.headline, 'No Read Allocations');
  });
});

// Namespace test: details shows the namespace
module('Acceptance | volume detail (with namespaces)', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  let volume;

  hooks.beforeEach(function() {
    server.createList('namespace', 2);
    server.create('node');
    server.create('csi-plugin', { createVolumes: false });
    volume = server.create('csi-volume');
  });

  test('/csi/volumes/:id detail ribbon includes the namespace of the volume', async function(assert) {
    await VolumeDetail.visit({ id: volume.id });

    assert.ok(VolumeDetail.hasNamespace);
    assert.ok(VolumeDetail.namespace.includes(volume.namespaceId || 'default'));
  });
});
