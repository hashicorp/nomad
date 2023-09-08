/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { next } from '@ember/runloop';
import { settled } from '@ember/test-helpers';
import { setupTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { AbortController } from 'fetch';

module('Unit | Adapter | Volume', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('volume');

    window.sessionStorage.clear();
    window.localStorage.clear();

    this.server = startMirage();

    this.initializeUI = async () => {
      this.server.create('namespace');
      this.server.create('namespace', { id: 'some-namespace' });
      this.server.create('node-pool');
      this.server.create('node');
      this.server.create('job', { id: 'job-1', namespaceId: 'default' });
      this.server.create('csi-plugin', 2);
      this.server.create('csi-volume', {
        id: 'volume-1',
        namespaceId: 'some-namespace',
      });

      this.server.create('region', { id: 'region-1' });
      this.server.create('region', { id: 'region-2' });

      this.system = this.owner.lookup('service:system');

      // Namespace, default region, and all regions are requests that all
      // job requests depend on. Fetching them ahead of time means testing
      // job adapter behavior in isolation.
      await this.system.get('namespaces');
      this.system.get('shouldIncludeRegion');
      await this.system.get('defaultRegion');

      // Reset the handledRequests array to avoid accounting for this
      // namespaces request everywhere.
      this.server.pretender.handledRequests.length = 0;
    };
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  test('The volume endpoint can be queried by type', async function (assert) {
    const { pretender } = this.server;

    await this.initializeUI();

    this.subject().query(
      this.store,
      { modelName: 'volume' },
      { type: 'csi' },
      null,
      {}
    );
    await settled();

    assert.deepEqual(pretender.handledRequests.mapBy('url'), [
      '/v1/volumes?type=csi',
    ]);
  });

  test('When the volume has a namespace other than default, it is in the URL', async function (assert) {
    const { pretender } = this.server;
    const volumeName = 'csi/volume-1';
    const volumeNamespace = 'some-namespace';
    const volumeId = JSON.stringify([volumeName, volumeNamespace]);

    await this.initializeUI();

    this.subject().findRecord(this.store, { modelName: 'volume' }, volumeId);
    await settled();

    assert.deepEqual(pretender.handledRequests.mapBy('url'), [
      `/v1/volume/${encodeURIComponent(
        volumeName
      )}?namespace=${volumeNamespace}`,
    ]);
  });

  test('query can be watched', async function (assert) {
    await this.initializeUI();

    const { pretender } = this.server;

    const request = () =>
      this.subject().query(
        this.store,
        { modelName: 'volume' },
        { type: 'csi' },
        null,
        {
          reload: true,
          adapterOptions: { watch: true },
        }
      );

    request();
    assert.equal(
      pretender.handledRequests[0].url,
      '/v1/volumes?type=csi&index=1'
    );

    await settled();
    request();
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/volumes?type=csi&index=2'
    );

    await settled();
  });

  test('query can be canceled', async function (assert) {
    await this.initializeUI();

    const { pretender } = this.server;
    const controller = new AbortController();

    pretender.get('/v1/volumes', () => [200, {}, '[]'], true);

    this.subject()
      .query(this.store, { modelName: 'volume' }, { type: 'csi' }, null, {
        reload: true,
        adapterOptions: { watch: true, abortController: controller },
      })
      .catch(() => {});

    const { request: xhr } = pretender.requestReferences[0];
    assert.equal(xhr.status, 0, 'Request is still pending');

    // Schedule the cancelation before waiting
    next(() => {
      controller.abort();
    });

    await settled();
    assert.ok(xhr.aborted, 'Request was aborted');
  });

  test('query and findAll have distinct watchList entries', async function (assert) {
    await this.initializeUI();

    const { pretender } = this.server;

    const request = () =>
      this.subject().query(
        this.store,
        { modelName: 'volume' },
        { type: 'csi' },
        null,
        {
          reload: true,
          adapterOptions: { watch: true },
        }
      );

    const findAllRequest = () =>
      this.subject().findAll(null, { modelName: 'volume' }, null, {
        reload: true,
        adapterOptions: { watch: true },
      });

    request();
    assert.equal(
      pretender.handledRequests[0].url,
      '/v1/volumes?type=csi&index=1'
    );

    await settled();
    request();
    assert.equal(
      pretender.handledRequests[1].url,
      '/v1/volumes?type=csi&index=2'
    );

    await settled();
    findAllRequest();
    assert.equal(pretender.handledRequests[2].url, '/v1/volumes?index=1');
  });
});
