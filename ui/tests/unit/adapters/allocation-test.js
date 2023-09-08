/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Unit | Adapter | Allocation', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('allocation');

    window.localStorage.clear();

    this.server = startMirage();

    this.initialize = async (allocationId, { region } = {}) => {
      if (region) window.localStorage.nomadActiveRegion = region;

      this.server.create('namespace');
      this.server.create('region', { id: 'region-1' });
      this.server.create('region', { id: 'region-2' });

      this.server.create('node-pool');
      this.server.create('node');
      this.server.create('job', { createAllocations: false });
      this.server.create('allocation', { id: 'alloc-1' });
      this.system = this.owner.lookup('service:system');
      await this.system.get('namespaces');
      this.system.get('shouldIncludeRegion');
      await this.system.get('defaultRegion');

      const allocation = await this.store.findRecord(
        'allocation',
        allocationId
      );
      this.server.pretender.handledRequests.length = 0;

      return allocation;
    };
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const testCases = [
    {
      variation: '',
      id: 'alloc-1',
      task: 'task-name',
      region: null,
      path: 'some/path',
      ls: `GET /v1/client/fs/ls/alloc-1?path=${encodeURIComponent(
        'some/path'
      )}`,
      stat: `GET /v1/client/fs/stat/alloc-1?path=${encodeURIComponent(
        'some/path'
      )}`,
      stop: 'POST /v1/allocation/alloc-1/stop',
      restart: 'PUT /v1/client/allocation/alloc-1/restart',
    },
    {
      variation: 'with non-default region',
      id: 'alloc-1',
      task: 'task-name',
      region: 'region-2',
      path: 'some/path',
      ls: `GET /v1/client/fs/ls/alloc-1?path=${encodeURIComponent(
        'some/path'
      )}&region=region-2`,
      stat: `GET /v1/client/fs/stat/alloc-1?path=${encodeURIComponent(
        'some/path'
      )}&region=region-2`,
      stop: 'POST /v1/allocation/alloc-1/stop?region=region-2',
      restart: 'PUT /v1/client/allocation/alloc-1/restart?region=region-2',
    },
  ];

  testCases.forEach((testCase) => {
    test(`ls makes the correct API call ${testCase.variation}`, async function (assert) {
      const { pretender } = this.server;
      const allocation = await this.initialize(testCase.id, {
        region: testCase.region,
      });

      await this.subject().ls(allocation, testCase.path);
      const req = pretender.handledRequests[0];
      assert.equal(`${req.method} ${req.url}`, testCase.ls);
    });

    test(`stat makes the correct API call ${testCase.variation}`, async function (assert) {
      const { pretender } = this.server;
      const allocation = await this.initialize(testCase.id, {
        region: testCase.region,
      });

      await this.subject().stat(allocation, testCase.path);
      const req = pretender.handledRequests[0];
      assert.equal(`${req.method} ${req.url}`, testCase.stat);
    });

    test(`stop makes the correct API call ${testCase.variation}`, async function (assert) {
      const { pretender } = this.server;
      const allocation = await this.initialize(testCase.id, {
        region: testCase.region,
      });

      await this.subject().stop(allocation);
      const req = pretender.handledRequests[0];
      assert.equal(`${req.method} ${req.url}`, testCase.stop);
    });

    test(`restart makes the correct API call ${testCase.variation}`, async function (assert) {
      const { pretender } = this.server;
      const allocation = await this.initialize(testCase.id, {
        region: testCase.region,
      });

      await this.subject().restart(allocation);
      const req = pretender.handledRequests[0];
      assert.equal(`${req.method} ${req.url}`, testCase.restart);
    });

    test(`restart with optional task name makes the correct API call ${testCase.variation}`, async function (assert) {
      const { pretender } = this.server;
      const allocation = await this.initialize(testCase.id, {
        region: testCase.region,
      });

      await this.subject().restart(allocation, testCase.task);
      const req = pretender.handledRequests[0];
      assert.equal(`${req.method} ${req.url}`, testCase.restart);
      assert.deepEqual(JSON.parse(req.requestBody), {
        TaskName: testCase.task,
      });
    });
  });
});
