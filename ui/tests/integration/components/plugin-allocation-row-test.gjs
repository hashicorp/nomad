/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { startMirage } from 'nomad-ui/tests/helpers/start-mirage';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import PluginAllocationRow from 'nomad-ui/components/plugin-allocation-row';

module('Integration | Component | plugin allocation row', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('node-pool');
    this.server.create('node');
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  test('Plugin allocation row immediately fetches the plugin allocation', async function (assert) {
    const plugin = this.server.create('csi-plugin', {
      id: 'plugin',
      controllerRequired: true,
    });
    const storageController = plugin.controllers.models[0];

    const pluginRecord = await this.store.find('plugin', 'csi/plugin');

    this.setProperties({
      plugin: pluginRecord.get('controllers.firstObject'),
    });

    await render(
      <template>
        <table><tbody>
            <PluginAllocationRow @pluginAllocation={{this.plugin}} />
          </tbody></table>
      </template>,
    );

    const allocationRequest = this.server.pretender.handledRequests.find(
      (request) => request.url.startsWith('/v1/allocation'),
    );
    assert.deepEqual(
      allocationRequest.url,
      `/v1/allocation/${storageController.allocID}`,
    );
    await componentA11yAudit(this.element, assert);
  });

  test('After the plugin allocation row fetches the plugin allocation, allocation stats are fetched', async function (assert) {
    const plugin = this.server.create('csi-plugin', {
      id: 'plugin',
      controllerRequired: true,
    });
    const storageController = plugin.controllers.models[0];

    const pluginRecord = await this.store.find('plugin', 'csi/plugin');

    this.setProperties({
      plugin: pluginRecord.get('controllers.firstObject'),
    });

    await render(
      <template>
        <table><tbody>
            <PluginAllocationRow @pluginAllocation={{this.plugin}} />
          </tbody></table>
      </template>,
    );

    const [statsRequest] = this.server.pretender.handledRequests.slice(-1);

    assert.deepEqual(
      statsRequest.url,
      `/v1/client/allocation/${storageController.allocID}/stats`,
    );
  });

  test('Setting a new plugin fetches the new plugin allocation', async function (assert) {
    const plugin = this.server.create('csi-plugin', {
      id: 'plugin',
      isMonolith: false,
      controllerRequired: true,
      controllersExpected: 2,
    });
    const storageController = plugin.controllers.models[0];
    const storageController2 = plugin.controllers.models[1];

    const pluginRecord = await this.store.find('plugin', 'csi/plugin');

    this.setProperties({
      plugin: pluginRecord.get('controllers.firstObject'),
    });

    await render(
      <template>
        <table><tbody>
            <PluginAllocationRow @pluginAllocation={{this.plugin}} />
          </tbody></table>
      </template>,
    );

    const allocationRequest = this.server.pretender.handledRequests.find(
      (request) => request.url.startsWith('/v1/allocation'),
    );

    assert.deepEqual(
      allocationRequest.url,
      `/v1/allocation/${storageController.allocID}`,
    );

    this.set('plugin', [...pluginRecord.get('controllers')][1]);
    await settled();

    const latestAllocationRequest = this.server.pretender.handledRequests
      .filter((request) => request.url.startsWith('/v1/allocation'))
      .reverse()[0];

    assert.deepEqual(
      latestAllocationRequest.url,
      `/v1/allocation/${storageController2.allocID}`,
    );
  });
});
