/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import NetworkModel from 'nomad-ui/models/network';

module('Unit | Serializer | Network', function (hooks) {
  setupTest(hooks);
  hooks.beforeEach(function () {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.serializerFor('network');
  });

  test('v4 IPs are passed through', async function (assert) {
    const ip = '10.0.13.12';
    const original = {
      IP: ip,
    };

    const { data } = this.subject().normalize(NetworkModel, original);
    assert.equal(data.attributes.ip, ip);
  });

  test('v6 IPs are wrapped in square brackets', async function (assert) {
    const ip = '2001:0dac:aba3:0000:0000:8a2e:0370:7334';
    const original = {
      IP: ip,
    };

    const { data } = this.subject().normalize(NetworkModel, original);
    assert.equal(data.attributes.ip, `[${ip}]`);
  });
});
