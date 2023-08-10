/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable qunit/require-expect */
import { currentURL, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import VolumesList from 'nomad-ui/tests/pages/storage/volumes/list';
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

module('Acceptance | volumes list', function (hooks) {
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
    await VolumesList.visit();
    await a11yAudit(assert);
  });

  test('visiting /csi redirects to /csi/volumes', async function (assert) {
    await visit('/csi');

    assert.equal(currentURL(), '/csi/volumes');
  });

  test('visiting /csi/volumes', async function (assert) {
    await VolumesList.visit();

    assert.equal(currentURL(), '/csi/volumes');
    assert.equal(document.title, 'CSI Volumes - Nomad');
  });

  test('/csi/volumes should list the first page of volumes sorted by name', async function (assert) {
    const volumeCount = VolumesList.pageSize + 1;
    server.createList('csi-volume', volumeCount);

    await VolumesList.visit();

    await percySnapshot(assert);

    const sortedVolumes = server.db.csiVolumes.sortBy('id');
    assert.equal(VolumesList.volumes.length, VolumesList.pageSize);
    VolumesList.volumes.forEach((volume, index) => {
      assert.equal(volume.name, sortedVolumes[index].id, 'Volumes are ordered');
    });
  });

  test('each volume row should contain information about the volume', async function (assert) {
    const volume = server.create('csi-volume');
    const readAllocs = server.createList('allocation', 2, { shallow: true });
    const writeAllocs = server.createList('allocation', 3, { shallow: true });
    readAllocs.forEach((alloc) => assignReadAlloc(volume, alloc));
    writeAllocs.forEach((alloc) => assignWriteAlloc(volume, alloc));

    await VolumesList.visit();

    const volumeRow = VolumesList.volumes.objectAt(0);

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
    assert.equal(volumeRow.provider, volume.provider);
    assert.equal(volumeRow.allocations, readAllocs.length + writeAllocs.length);
  });

  test('each volume row should link to the corresponding volume', async function (assert) {
    const [, secondNamespace] = server.createList('namespace', 2);
    const volume = server.create('csi-volume', {
      namespaceId: secondNamespace.id,
    });

    await VolumesList.visit({ namespace: '*' });

    await VolumesList.volumes.objectAt(0).clickName();
    assert.equal(
      currentURL(),
      `/csi/volumes/${volume.id}@${secondNamespace.id}`
    );

    await VolumesList.visit({ namespace: '*' });
    assert.equal(currentURL(), '/csi/volumes?namespace=*');

    await VolumesList.volumes.objectAt(0).clickRow();
    assert.equal(
      currentURL(),
      `/csi/volumes/${volume.id}@${secondNamespace.id}`
    );
  });

  test('when there are no volumes, there is an empty message', async function (assert) {
    await VolumesList.visit();

    await percySnapshot(assert);

    assert.ok(VolumesList.isEmpty);
    assert.equal(VolumesList.emptyState.headline, 'No Volumes');
  });

  test('when there are volumes, but no matches for a search, there is an empty message', async function (assert) {
    server.create('csi-volume', { id: 'cat 1' });
    server.create('csi-volume', { id: 'cat 2' });

    await VolumesList.visit();

    await VolumesList.search('dog');
    assert.ok(VolumesList.isEmpty);
    assert.equal(VolumesList.emptyState.headline, 'No Matches');
  });

  test('searching resets the current page', async function (assert) {
    server.createList('csi-volume', VolumesList.pageSize + 1);

    await VolumesList.visit();
    await VolumesList.nextPage();

    assert.equal(currentURL(), '/csi/volumes?page=2');

    await VolumesList.search('foobar');

    assert.equal(currentURL(), '/csi/volumes?search=foobar');
  });

  test('when the cluster has namespaces, each volume row includes the volume namespace', async function (assert) {
    server.createList('namespace', 2);
    const volume = server.create('csi-volume');

    await VolumesList.visit({ namespace: '*' });

    const volumeRow = VolumesList.volumes.objectAt(0);
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

    await VolumesList.visit();
    assert.equal(VolumesList.volumes.length, 2);

    const firstNamespace = server.db.namespaces[0];
    await VolumesList.visit({ namespace: firstNamespace.id });
    assert.equal(VolumesList.volumes.length, 1);
    assert.equal(VolumesList.volumes.objectAt(0).name, volume1.id);

    const secondNamespace = server.db.namespaces[1];
    await VolumesList.visit({ namespace: secondNamespace.id });

    assert.equal(VolumesList.volumes.length, 1);
    assert.equal(VolumesList.volumes.objectAt(0).name, volume2.id);
  });

  test('when accessing volumes is forbidden, a message is shown with a link to the tokens page', async function (assert) {
    server.pretender.get('/v1/volumes', () => [403, {}, null]);

    await VolumesList.visit();
    assert.equal(VolumesList.error.title, 'Not Authorized');

    await VolumesList.error.seekHelp();
    assert.equal(currentURL(), '/settings/tokens');
  });

  pageSizeSelect({
    resourceName: 'volume',
    pageObject: VolumesList,
    pageObjectList: VolumesList.volumes,
    async setup() {
      server.createList('csi-volume', VolumesList.pageSize);
      await VolumesList.visit();
    },
  });

  testSingleSelectFacet('Namespace', {
    facet: VolumesList.facets.namespace,
    paramName: 'namespace',
    expectedOptions: ['All (*)', 'default', 'namespace-2'],
    optionToSelect: 'namespace-2',
    async beforeEach() {
      server.create('namespace', { id: 'default' });
      server.create('namespace', { id: 'namespace-2' });
      server.createList('csi-volume', 2, { namespaceId: 'default' });
      server.createList('csi-volume', 2, { namespaceId: 'namespace-2' });
      await VolumesList.visit();
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
      const selection = option.key;
      await option.select();

      const expectedVolumes = server.db.csiVolumes
        .filter((volume) => filter(volume, selection))
        .sortBy('id');

      VolumesList.volumes.forEach((volume, index) => {
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
      const selection = option.key;
      await option.select();

      assert.ok(
        currentURL().includes(`${paramName}=${selection}`),
        'URL has the correct query param key and value'
      );
    });
  }
});
