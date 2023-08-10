/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Unit | Adapter | Deployment', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function () {
    this.store = this.owner.lookup('service:store');
    this.system = this.owner.lookup('service:system');
    this.subject = () => this.store.adapterFor('deployment');

    window.localStorage.clear();

    this.server = startMirage();

    this.initialize = async ({ region } = {}) => {
      if (region) window.localStorage.nomadActiveRegion = region;

      this.server.create('region', { id: 'region-1' });
      this.server.create('region', { id: 'region-2' });

      this.server.create('node-pool');
      this.server.create('node');
      const job = this.server.create('job', { createAllocations: false });
      const deploymentRecord = server.schema.deployments.where({
        jobId: job.id,
      }).models[0];

      this.system.get('shouldIncludeRegion');
      await this.system.get('defaultRegion');

      const deployment = await this.store.findRecord(
        'deployment',
        deploymentRecord.id
      );
      this.server.pretender.handledRequests.length = 0;

      return deployment;
    };
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const testCases = [
    {
      variation: '',
      region: null,
      fail: (id) => `POST /v1/deployment/fail/${id}`,
      promote: (id) => `POST /v1/deployment/promote/${id}`,
    },
    {
      variation: 'with non-default region',
      region: 'region-2',
      fail: (id) => `POST /v1/deployment/fail/${id}?region=region-2`,
      promote: (id) => `POST /v1/deployment/promote/${id}?region=region-2`,
    },
  ];

  testCases.forEach((testCase) => {
    test(`promote makes the correct API call ${testCase.variation}`, async function (assert) {
      const deployment = await this.initialize({ region: testCase.region });
      await this.subject().promote(deployment);

      const request = this.server.pretender.handledRequests[0];

      assert.equal(
        `${request.method} ${request.url}`,
        testCase.promote(deployment.id)
      );
      assert.deepEqual(JSON.parse(request.requestBody), {
        DeploymentId: deployment.id,
        All: true,
      });
    });

    test(`fail makes the correct API call ${testCase.variation}`, async function (assert) {
      const deployment = await this.initialize({ region: testCase.region });
      await this.subject().fail(deployment);

      const request = this.server.pretender.handledRequests[0];

      assert.equal(
        `${request.method} ${request.url}`,
        testCase.fail(deployment.id)
      );
      assert.deepEqual(JSON.parse(request.requestBody), {
        DeploymentId: deployment.id,
      });
    });
  });
});
