import { currentURL, visit } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import a11yAudit from 'nomad-ui/tests/helpers/a11y-audit';
import pageSizeSelect from './behaviors/page-size-select';
import VolumesList from 'nomad-ui/tests/pages/storage/volumes/list';
import Layout from 'nomad-ui/tests/pages/layout';

const assignWriteAlloc = (volume, alloc) => {
  volume.writeAllocs.add(alloc);
  volume.save();
};

const assignReadAlloc = (volume, alloc) => {
  volume.readAllocs.add(alloc);
  volume.save();
};

module('Acceptance | volumes list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {
    server.create('node');
    server.create('csi-plugin', { createVolumes: false });
    window.localStorage.clear();
  });

  test('it passes an accessibility audit', async function(assert) {
    await VolumesList.visit();
    await a11yAudit(assert);
  });

  test('visiting /csi redirects to /csi/volumes', async function(assert) {
    await visit('/csi');

    assert.equal(currentURL(), '/csi/volumes');
  });

  test('visiting /csi/volumes', async function(assert) {
    await VolumesList.visit();

    assert.equal(currentURL(), '/csi/volumes');
    assert.equal(document.title, 'CSI Volumes - Nomad');
  });

  test('/csi/volumes should list the first page of volumes sorted by name', async function(assert) {
    const volumeCount = VolumesList.pageSize + 1;
    server.createList('csi-volume', volumeCount);

    await VolumesList.visit();

    const sortedVolumes = server.db.csiVolumes.sortBy('id');
    assert.equal(VolumesList.volumes.length, VolumesList.pageSize);
    VolumesList.volumes.forEach((volume, index) => {
      assert.equal(volume.name, sortedVolumes[index].id, 'Volumes are ordered');
    });
  });

  test('each volume row should contain information about the volume', async function(assert) {
    const volume = server.create('csi-volume');
    const readAllocs = server.createList('allocation', 2, { shallow: true });
    const writeAllocs = server.createList('allocation', 3, { shallow: true });
    readAllocs.forEach(alloc => assignReadAlloc(volume, alloc));
    writeAllocs.forEach(alloc => assignWriteAlloc(volume, alloc));

    await VolumesList.visit();

    const volumeRow = VolumesList.volumes.objectAt(0);

    const controllerHealthStr = volume.controllersHealthy > 0 ? 'Healthy' : 'Unhealthy';
    const nodeHealthStr = volume.nodesHealthy > 0 ? 'Healthy' : 'Unhealthy';

    assert.equal(volumeRow.name, volume.id);
    assert.equal(volumeRow.schedulable, volume.schedulable ? 'Schedulable' : 'Unschedulable');
    assert.equal(
      volumeRow.controllerHealth,
      `${controllerHealthStr} (${volume.controllersHealthy}/${volume.controllersExpected})`
    );
    assert.equal(
      volumeRow.nodeHealth,
      `${nodeHealthStr} (${volume.nodesHealthy}/${volume.nodesExpected})`
    );
    assert.equal(volumeRow.provider, volume.provider);
    assert.equal(volumeRow.allocations, readAllocs.length + writeAllocs.length);
  });

  test('each volume row should link to the corresponding volume', async function(assert) {
    const volume = server.create('csi-volume');

    await VolumesList.visit();

    await VolumesList.volumes.objectAt(0).clickName();
    assert.equal(currentURL(), `/csi/volumes/${volume.id}`);

    await VolumesList.visit();
    assert.equal(currentURL(), '/csi/volumes');

    await VolumesList.volumes.objectAt(0).clickRow();
    assert.equal(currentURL(), `/csi/volumes/${volume.id}`);
  });

  test('when there are no volumes, there is an empty message', async function(assert) {
    await VolumesList.visit();

    assert.ok(VolumesList.isEmpty);
    assert.equal(VolumesList.emptyState.headline, 'No Volumes');
  });

  test('when there are volumes, but no matches for a search, there is an empty message', async function(assert) {
    server.create('csi-volume', { id: 'cat 1' });
    server.create('csi-volume', { id: 'cat 2' });

    await VolumesList.visit();

    await VolumesList.search('dog');
    assert.ok(VolumesList.isEmpty);
    assert.equal(VolumesList.emptyState.headline, 'No Matches');
  });

  test('searching resets the current page', async function(assert) {
    server.createList('csi-volume', VolumesList.pageSize + 1);

    await VolumesList.visit();
    await VolumesList.nextPage();

    assert.equal(currentURL(), '/csi/volumes?page=2');

    await VolumesList.search('foobar');

    assert.equal(currentURL(), '/csi/volumes?search=foobar');
  });

  test('when the namespace query param is set, only matching volumes are shown and the namespace value is forwarded to app state', async function(assert) {
    server.createList('namespace', 2);
    const volume1 = server.create('csi-volume', { namespaceId: server.db.namespaces[0].id });
    const volume2 = server.create('csi-volume', { namespaceId: server.db.namespaces[1].id });

    await VolumesList.visit();

    assert.equal(VolumesList.volumes.length, 1);
    assert.equal(VolumesList.volumes.objectAt(0).name, volume1.id);

    const secondNamespace = server.db.namespaces[1];
    await VolumesList.visit({ namespace: secondNamespace.id });

    assert.equal(VolumesList.volumes.length, 1);
    assert.equal(VolumesList.volumes.objectAt(0).name, volume2.id);
  });

  test('the active namespace is carried over to the jobs pages', async function(assert) {
    server.createList('namespace', 2);

    const namespace = server.db.namespaces[1];
    await VolumesList.visit({ namespace: namespace.id });

    await Layout.gutter.visitJobs();

    assert.equal(currentURL(), `/jobs?namespace=${namespace.id}`);
  });

  test('when accessing volumes is forbidden, a message is shown with a link to the tokens page', async function(assert) {
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
});
