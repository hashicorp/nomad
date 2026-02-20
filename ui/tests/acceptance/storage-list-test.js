/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import StorageList from 'nomad-ui/tests/pages/storage/list';
import percySnapshot from '@percy/ember';
import faker from 'nomad-ui/mirage/faker';

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

module('Acceptance | storage list', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    faker.seed(1);
    server.create('node-pool');
    server.create('node');
    server.create('csi-plugin', { createVolumes: false });
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function (assert) {
    await StorageList.visit();
    await a11yAudit(assert);
  });

  test('visiting the now-deprecated /csi redirects to /storage', async function (assert) {
    await visit('/csi');

    assert.equal(currentURL(), '/storage');
  });

  test('visiting /storage', async function (assert) {
    await StorageList.visit();

    assert.equal(currentURL(), '/storage');
    assert.equal(document.title, 'Storage - Nomad');
  });

  test('/storage/volumes should list the first page of volumes sorted by name', async function (assert) {
    const volumeCount = StorageList.pageSize + 1;
    server.createList('csi-volume', volumeCount);

    await StorageList.visit();

    await percySnapshot(assert);

    const sortedVolumes = server.db.csiVolumes.sortBy('id');

    assert.equal(StorageList.csiVolumes.length, StorageList.pageSize);
    StorageList.csiVolumes.forEach((volume, index) => {
      assert.equal(volume.name, sortedVolumes[index].id, 'Volumes are ordered');
    });
  });

  test('each volume row should contain information about the volume', async function (assert) {
    const volume = server.create('csi-volume');
    const readAllocs = server.createList('allocation', 2, { shallow: true });
    const writeAllocs = server.createList('allocation', 3, { shallow: true });
    readAllocs.forEach((alloc) => assignReadAlloc(volume, alloc));
    writeAllocs.forEach((alloc) => assignWriteAlloc(volume, alloc));

    await StorageList.visit();

    const volumeRow = StorageList.csiVolumes.objectAt(0);

    let controllerHealthStr = 'Node Only';
    if (volume.controllerRequired || volume.controllersExpected > 0) {
      const healthy = volume.controllersHealthy;
      const expected = volume.controllersExpected;
      const isHealthy = healthy > 0;
      controllerHealthStr = `${
        isHealthy ? 'Healthy' : 'Unhealthy'
      } ( ${healthy} / ${expected} )`;
    }

    const nodeHealthStr = volume.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.equal(volumeRow.name, volume.id);
    assert.notOk(volumeRow.hasNamespace);
    assert.equal(
      volumeRow.schedulable,
      volume.schedulable ? 'Schedulable' : 'Unschedulable'
    );
    assert.equal(volumeRow.controllerHealth, controllerHealthStr);
    assert.equal(
      volumeRow.nodeHealth,
      `${nodeHealthStr} ( ${volume.nodesHealthy} / ${volume.nodesExpected} )`
    );
    assert.equal(volumeRow.plugin, volume.PluginId);
    assert.equal(volumeRow.allocations, readAllocs.length + writeAllocs.length);
  });

  test('each volume row should link to the corresponding volume', async function (assert) {
    const [, secondNamespace] = server.createList('namespace', 2);
    const volume = server.create('csi-volume', {
      namespaceId: secondNamespace.id,
    });

    await StorageList.visit({ namespace: '*' });
    await StorageList.csiVolumes.objectAt(0).clickName();
    assert.equal(
      currentURL(),
      `/storage/volumes/csi/${volume.id}@${secondNamespace.id}`
    );

    await StorageList.visit({ namespace: '*' });
    assert.equal(currentURL(), '/storage');
  });

  test('when there are no csi volumes, there is an empty message', async function (assert) {
    await StorageList.visit();

    await percySnapshot(assert);

    assert.ok(StorageList.csiIsEmpty);
    assert.equal(StorageList.csiEmptyState, 'No CSI Volumes found');
  });

  test('when there are volumes, but no matches for a search, there is an empty message', async function (assert) {
    server.create('csi-volume', { id: 'cat 1' });
    server.create('csi-volume', { id: 'cat 2' });

    await StorageList.visit();
    await StorageList.csiSearch('dog');
    assert.ok(StorageList.csiIsEmpty);
    assert.ok(
      StorageList.csiEmptyState.includes('No CSI volumes match your search')
    );
  });

  test('searching resets the current page', async function (assert) {
    server.createList('csi-volume', StorageList.pageSize + 1);

    await StorageList.visit();
    await StorageList.csiNextPage();

    assert.equal(currentURL(), '/storage?csiPage=2');

    await StorageList.csiSearch('foobar');

    assert.equal(currentURL(), '/storage?csiFilter=foobar');
  });

  test('when the cluster has namespaces, each volume row includes the volume namespace', async function (assert) {
    server.createList('namespace', 2);
    const volume = server.create('csi-volume');

    await StorageList.visit({ namespace: '*' });

    const volumeRow = StorageList.csiVolumes.objectAt(0);
    assert.equal(volumeRow.namespace, volume.namespaceId);
  });

  test('when the namespace query param is set, only matching volumes are shown and the namespace value is forwarded to app state', async function (assert) {
    server.createList('namespace', 2);
    const volume1 = server.create('csi-volume', {
      namespaceId: server.db.namespaces[0].id,
    });
    const volume2 = server.create('csi-volume', {
      namespaceId: server.db.namespaces[1].id,
    });

    await StorageList.visit();
    assert.equal(StorageList.csiVolumes.length, 2);

    const firstNamespace = server.db.namespaces[0];
    await StorageList.visit({ namespace: firstNamespace.id });
    assert.equal(StorageList.csiVolumes.length, 1);
    assert.equal(StorageList.csiVolumes.objectAt(0).name, volume1.id);

    const secondNamespace = server.db.namespaces[1];
    await StorageList.visit({ namespace: secondNamespace.id });

    assert.equal(StorageList.csiVolumes.length, 1);
    assert.equal(StorageList.csiVolumes.objectAt(0).name, volume2.id);
  });

  test('when accessing volumes is forbidden, a message is shown with a link to the tokens page', async function (assert) {
    server.pretender.get('/v1/volumes', () => [403, {}, null]);

    await StorageList.visit();
    assert.equal(StorageList.error.title, 'Not Authorized');

    await StorageList.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });

  testSingleSelectFacet('Namespace', {
    facet: StorageList.facets.namespace,
    paramName: 'namespace',
    expectedOptions: ['All (*)', 'default', 'namespace-2'],
    optionToSelect: 'namespace-2',
    async beforeEach() {
      server.create('namespace', { id: 'default' });
      server.create('namespace', { id: 'namespace-2' });
      server.createList('csi-volume', 2, { namespaceId: 'default' });
      server.createList('csi-volume', 2, { namespaceId: 'namespace-2' });
      await StorageList.visit();
    },
    filter(volume, selection) {
      return volume.namespaceId === selection;
    },
  });

  function testSingleSelectFacet(
    label,
    { facet, paramName, beforeEach, filter, expectedOptions, optionToSelect }
  ) {
    test(`the ${label} facet has the correct options`, async function (assert) {
      await beforeEach();
      await facet.toggle();

      let expectation;
      if (typeof expectedOptions === 'function') {
        expectation = expectedOptions(server.db.jobs);
      } else {
        expectation = expectedOptions;
      }

      assert.deepEqual(
        facet.options.map((option) => option.label.trim()),
        expectation,
        'Options for facet are as expected'
      );
    });

    test(`the ${label} facet filters the volumes list by ${label}`, async function (assert) {
      await beforeEach();
      await facet.toggle();

      const option = facet.options.findOneBy('label', optionToSelect);
      const selection = option.label;
      await option.toggle();

      const expectedVolumes = server.db.csiVolumes
        .filter((volume) => filter(volume, selection))
        .sortBy('id');

      StorageList.csiVolumes.forEach((volume, index) => {
        assert.equal(
          volume.name,
          expectedVolumes[index].name,
          `Volume at ${index} is ${expectedVolumes[index].name}`
        );
      });
    });

    test(`selecting an option in the ${label} facet updates the ${paramName} query param`, async function (assert) {
      await beforeEach();
      await facet.toggle();

      const option = facet.options.objectAt(1);
      const label = option.label;
      await option.toggle();

      assert.ok(
        currentURL().includes(`${paramName}=${label}`),
        'URL has the correct query param key and value'
      );
    });

    module('Live updates are reflected in the list', function () {
      test('When you visit the storage list page, the watch process is kicked off', async function (assert) {
        await StorageList.visit();
        const requests = server.pretender.handledRequests;
        const dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        const csiRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=csi')
        );
        assert.equal(dhvRequests.length, 2, '2 DHV requests were made');
        assert.equal(csiRequests.length, 2, '2 CSI requests were made');
      });

      test('When a new dynamic host volume is created, the page should reflect the changes', async function (assert) {
        server.create('dynamic-host-volume', {
          name: 'initial-volume',
        });
        const controller = this.owner.lookup('controller:storage.index');
        await visit('/storage');

        // Check pretender to see 2 requests related to DHV: the initial one, and another one with an index on it
        const requests = server.pretender.handledRequests;

        // Should be 2 DHV requests made: the initial one, and the watcher
        let dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 2, '2 DHV requests were made');

        assert.dom('[data-test-dhv-row]').exists({ count: 1 });
        assert.dom('[data-test-dhv-row]').containsText('initial-volume');

        server.create('dynamic-host-volume', {
          name: 'new-volume',
        });

        await controller.watchDHV.perform({
          type: 'host',
          namespace: controller.qpNamespace,
        });

        // Now there should be a third DHV request
        dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 3, '3 DHV requests were made');

        // and a second row
        assert.dom('[data-test-dhv-row]').exists({ count: 2 });
      });

      test('When a new csi volume is created, the page should reflect the changes', async function (assert) {
        server.create('csi-volume', {
          id: 'initial-volume',
        });
        const controller = this.owner.lookup('controller:storage.index');
        await visit('/storage');

        // Check pretender to see 2 requests related to DHV: the initial one, and another one with an index on it
        const requests = server.pretender.handledRequests;

        // Should be 2 DHV requests made: the initial one, and the watcher
        let csiRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=csi')
        );
        assert.equal(csiRequests.length, 2, '2 CSI requests were made');
        assert.dom('[data-test-csi-volume-row]').exists({ count: 1 });
        assert.dom('[data-test-csi-volume-row]').containsText('initial-volume');

        server.create('csi-volume', {
          id: 'new-volume',
        });

        await controller.watchCSI.perform({
          type: 'csi',
          namespace: controller.qpNamespace,
        });

        // Now there should be a third DHV request
        csiRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=csi')
        );
        assert.equal(csiRequests.length, 3, '3 CSI requests were made');

        // and a second row
        assert.dom('[data-test-csi-volume-row]').exists({ count: 2 });
      });

      test('When a dynamic host volume is updated, the page should reflect the changes', async function (assert) {
        const dhv = server.create('dynamic-host-volume', {
          name: 'initial-volume',
        });
        const controller = this.owner.lookup('controller:storage.index');
        await visit('/storage');

        // Check pretender to see 2 requests related to DHV: the initial one, and another one with an index on it
        const requests = server.pretender.handledRequests;

        // Should be 2 DHV requests made: the initial one, and the watcher
        let dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 2, '2 DHV requests were made');

        assert.dom('[data-test-dhv-row]').exists({ count: 1 });
        assert.dom('[data-test-dhv-row]').containsText('initial-volume');

        dhv.update('name', 'updated-volume');

        await controller.watchDHV.perform({
          type: 'host',
          namespace: controller.qpNamespace,
        });

        dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 3, '3 DHV requests were made');

        // Still just one row
        assert.dom('[data-test-dhv-row]').exists({ count: 1 });
        assert.dom('[data-test-dhv-row]').containsText('updated-volume');
      });

      test('When a dynamic host volume is deleted, the page should reflect the changes', async function (assert) {
        const dhv = server.create('dynamic-host-volume', {
          name: 'initial-volume',
        });
        const controller = this.owner.lookup('controller:storage.index');
        await visit('/storage');

        // Check pretender to see 2 requests related to DHV: the initial one, and another one with an index on it
        const requests = server.pretender.handledRequests;

        // Should be 2 DHV requests made: the initial one, and the watcher
        let dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 2, '2 DHV requests were made');

        assert.dom('[data-test-dhv-row]').exists({ count: 1 });
        assert.dom('[data-test-dhv-row]').containsText('initial-volume');

        dhv.destroy();

        await controller.watchDHV.perform({
          type: 'host',
          namespace: controller.qpNamespace,
        });

        dhvRequests = requests.filter((request) =>
          request.url.startsWith('/v1/volumes?namespace=%2A&type=host')
        );
        assert.equal(dhvRequests.length, 3, '3 DHV requests were made');

        assert.dom('[data-test-dhv-row]').exists({ count: 0 });
        assert.ok(StorageList.dhvIsEmpty);
      });
    });

    test('Pagination is adhered to when live updates happen', async function (assert) {
      localStorage.setItem('nomadPageSize', 10);
      server.createList('dynamic-host-volume', 9);
      const controller = this.owner.lookup('controller:storage.index');

      await StorageList.visit();

      // 9 rows should be present
      assert.dom('[data-test-dhv-row]').exists({ count: 9 });

      // Use an explicit modifyTime in the future so this volume sorts first
      const futureTime = (Date.now() + 60000) * 1000000;
      server.create('dynamic-host-volume', {
        name: 'tenth-volume',
        modifyTime: futureTime,
      });

      await controller.watchDHV.perform({
        type: 'host',
        namespace: controller.qpNamespace,
      });

      // 10 rows should be present
      assert.dom('[data-test-dhv-row]').exists({ count: 10 });

      // Newest (sorted by modified date by default) should show up first
      assert
        .dom('[data-test-dhv-row]:first-child')
        .containsText('tenth-volume');

      // There should still only be 1 page of pagination
      assert.dom('.hds-pagination-nav__number').exists({ count: 1 });
      // 10 rows should be present
      assert.dom('[data-test-dhv-row]').exists({ count: 10 });

      // Create one more with an even newer modifyTime
      server.create('dynamic-host-volume', {
        name: 'eleventh-volume',
        modifyTime: futureTime + 60000 * 1000000,
      });

      await controller.watchDHV.perform({
        type: 'host',
        namespace: controller.qpNamespace,
      });

      // 10 rows still present
      assert.dom('[data-test-dhv-row]').exists({ count: 10 });

      // Newest should show up first
      assert
        .dom('[data-test-dhv-row]:first-child')
        .containsText('eleventh-volume');

      // There should now be 2 pages of pagination
      assert.dom('.hds-pagination-nav__number').exists({ count: 2 });

      // Clicking through to the second page changes the URL and only shows 1 row
      await StorageList.dhvNextPage();
      assert.equal(currentURL(), '/storage?dhvPage=2');

      // 1 row should be present
      assert.dom('[data-test-dhv-row]').exists({ count: 1 });

      // cleanup
      localStorage.removeItem('nomadPageSize');
    });
  }
});
