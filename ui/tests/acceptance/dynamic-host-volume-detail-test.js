/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { a11yAudit } from 'ember-a11y-testing/test-support';
import { currentURL } from '@ember/test-helpers';
import { getPageTitle } from 'ember-page-title/test-support';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import setupAuthenticatedAcceptance from 'nomad-ui/tests/helpers/setup-authenticated-acceptance';
import moment from 'moment';
import { formatBytes, formatHertz } from 'nomad-ui/utils/units';
import VolumeDetail from 'nomad-ui/tests/pages/storage/dynamic-host-volumes/detail';
import Layout from 'nomad-ui/tests/pages/layout';
import faker from 'nomad-ui/mirage/faker';

const assignAlloc = (volume, alloc) => {
  volume.allocations.add(alloc);
  volume.save();
};

module('Acceptance | dynamic host volume detail', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);
  setupAuthenticatedAcceptance(hooks);

  let volume;

  hooks.beforeEach(function () {
    faker.seed(1);
    this.server.create('node-pool');
    this.server.create('node');
    this.server.create('job', {
      name: 'dhv-job',
    });
    volume = this.server.create('dynamic-host-volume', {
      nodeId: this.server.db.nodes[0].id,
    });
  });

  test('it passes an accessibility audit', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await a11yAudit();
    assert.ok(true, 'no a11y errors found');
  });

  test('/storage/volumes/:id should have a breadcrumb trail linking back to Volumes and Storage', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.deepEqual(Layout.breadcrumbFor('storage.index').text, 'Storage');
    assert.deepEqual(
      Layout.breadcrumbFor('storage.volumes.dynamic-host-volume').text,
      volume.name,
    );
  });

  test('/storage/volumes/:id should show the volume name in the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(
      getPageTitle().startsWith(`Dynamic Host Volume ${volume.name} - `),
      `title starts with the dynamic host volume name: ${getPageTitle()}`,
    );
    assert.ok(
      getPageTitle().endsWith(' - Nomad'),
      `title ends with Nomad branding: ${getPageTitle()}`,
    );
    assert.deepEqual(VolumeDetail.title, volume.name);
  });

  test('/storage/volumes/:id should list additional details for the volume below the title', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    assert.ok(VolumeDetail.node.includes(volume.node.name));
    assert.ok(VolumeDetail.plugin.includes(volume.pluginID));
    assert.notOk(
      VolumeDetail.hasNamespace,
      'Namespace is omitted when there is only one namespace',
    );
    assert.deepEqual(VolumeDetail.capacity, 'Capacity 9.54 MiB');
  });

  test('/storage/volumes/:id should list all allocations the volume is attached to', async function (assert) {
    // Use fixed timestamps so both absolute dates and relative times are
    // deterministic across Percy snapshot runs.
    const pinned = new Date('2025-06-15T12:00:00Z');
    const pinnedNs = pinned.getTime() * 1e6; // nanoseconds

    const allocations = [
      this.server.create('allocation', {
        createTime: pinnedNs - 9 * 3600e9,
        modifyTime: pinnedNs - 9 * 3600e9,
      }),
      this.server.create('allocation', {
        createTime: pinnedNs - 15 * 3600e9,
        modifyTime: pinnedNs - 15 * 3600e9,
      }),
      this.server.create('allocation', {
        createTime: pinnedNs - 1 * 3600e9,
        modifyTime: pinnedNs - 1 * 3600e9,
      }),
    ];
    allocations.forEach((alloc) => assignAlloc(volume, alloc));

    // Freeze moment's time reference so relative times ("9 hours ago") are
    // deterministic across Percy snapshot runs.
    const originalMomentNow = moment.now;
    moment.now = () => pinned.getTime();

    try {
      await VolumeDetail.visit({ id: `${volume.id}@default` });

      assert.deepEqual(VolumeDetail.allocations.length, allocations.length);
      allocations
        .sortBy('modifyIndex')
        .reverse()
        .forEach((allocation, idx) => {
          assert.deepEqual(
            allocation.id,
            VolumeDetail.allocations.objectAt(idx).id,
          );
        });
    } finally {
      moment.now = originalMomentNow;
    }
  });

  test('allocations show max run deadline when configured', async function (assert) {
    const maxRunDuration = 10 * 60 * 1000000000;
    const startedAt = new Date('2025-01-02T03:04:05Z');
    const expectedDeadline = new Date(
      startedAt.getTime() + maxRunDuration / 1000000,
    );

    const batchJob = this.server.create('job', {
      type: 'batch',
      createAllocations: false,
    });
    const taskGroup = this.server.db.taskGroups.findBy({ jobId: batchJob.id });
    this.server.db.taskGroups.update(taskGroup.id, { maxRunDuration });

    const allocation = this.server.create('allocation', {
      clientStatus: 'running',
      jobId: batchJob.id,
      taskGroup: taskGroup.name,
      modifyIndex: 999999,
    });
    assignAlloc(volume, allocation);

    this.server.db.taskStates
      .where({ allocationId: allocation.id })
      .forEach((taskState) => {
        this.server.db.taskStates.update(taskState.id, {
          state: 'running',
          startedAt,
        });
      });

    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.equal(
      VolumeDetail.allocationFor(allocation.id).maxRunDeadlineTooltip,
      moment(expectedDeadline).format("MMM DD, 'YY HH:mm:ss ZZ"),
      'The allocations table shows the computed max run deadline',
    );
  });

  test('each allocation should have high-level details for the allocation', async function (assert) {
    const allocation = this.server.create('allocation', {
      clientStatus: 'running',
    });
    assignAlloc(volume, allocation);

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
    VolumeDetail.allocations.objectAt(0).as((allocationRow) => {
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
    assignAlloc(volume, allocation);

    await VolumeDetail.visit({ id: `${volume.id}@default` });
    await VolumeDetail.allocations.objectAt(0).visit();

    assert.deepEqual(currentURL(), `/allocations/${allocation.id}`);
  });

  test('when there are no allocations, the table presents an empty state', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });

    assert.ok(VolumeDetail.allocationsTableIsEmpty);
    assert.deepEqual(
      VolumeDetail.allocationsEmptyState.headline,
      'No Allocations',
    );
  });

  test('Capabilities table shows access mode and attachment mode', async function (assert) {
    await VolumeDetail.visit({ id: `${volume.id}@default` });
    assert.deepEqual(
      VolumeDetail.capabilities.objectAt(0).accessMode,
      'single-node-writer',
    );
    assert.deepEqual(
      VolumeDetail.capabilities.objectAt(0).attachmentMode,
      'file-system',
    );
    assert.deepEqual(
      VolumeDetail.capabilities.objectAt(1).accessMode,
      'single-node-reader-only',
    );
    assert.deepEqual(
      VolumeDetail.capabilities.objectAt(1).attachmentMode,
      'block-device',
    );
  });
});

// Namespace test: details shows the namespace
module(
  'Acceptance | dynamic volume detail (with namespaces)',
  function (hooks) {
    setupApplicationTest(hooks);
    setupMirage(hooks);
    setupAuthenticatedAcceptance(hooks);

    let volume;

    hooks.beforeEach(function () {
      this.server.createList('namespace', 2);
      this.server.create('node-pool');
      this.server.create('node');
      volume = this.server.create('dynamic-host-volume');
    });

    test('/storage/volumes/:id detail ribbon includes the namespace of the volume', async function (assert) {
      await VolumeDetail.visit({ id: `${volume.id}@${volume.namespaceId}` });

      assert.ok(VolumeDetail.hasNamespace);
      assert.ok(
        VolumeDetail.namespace.includes(volume.namespaceId || 'default'),
      );
    });
  },
);
