/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import PortModel from 'nomad-ui/models/port';

module('Unit | Serializer | Port', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('port');
  });

  test('v4 HostIPs are passed through', async function (assert) {
    const ip = '10.0.13.12';
    const original = { HostIP: ip };

    const { data } = this.subject().normalize(PortModel, original);
    assert.deepEqual(data.attributes.hostIp, ip);
  });

  test('v6 HostIPs are wrapped in square brackets', async function (assert) {
    const ip = '2001:0dac:aba3:0000:0000:8a2e:0370:7334';
    const original = { HostIP: ip };

    const { data } = this.subject().normalize(PortModel, original);
    assert.deepEqual(data.attributes.hostIp, `[${ip}]`);
  });

  test('missing HostIP does not throw', async function (assert) {
    const original = { Label: 'http', Value: 80 };

    assert.deepEqual(
      this.subject().normalize(PortModel, original).data.attributes.hostIp,
      undefined,
    );
  });
});
