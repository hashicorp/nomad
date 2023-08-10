/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import Pretender from 'pretender';

module('Unit | Service | Token', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.subject = function () {
      return this.owner.factoryFor('service:token').create();
    };
  });

  hooks.beforeEach(function () {
    const mockSystem = Service.extend({
      activeRegion: 'region-1',
      shouldIncludeRegion: true,
    });

    this.owner.register('service:system', mockSystem);
    this.system = this.owner.lookup('service:system');

    this.server = new Pretender(function () {
      this.get('/path', () => [200, {}, null]);
    });
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  test('authorizedRequest includes the region param when the system service says to', function (assert) {
    const token = this.subject();

    token.authorizedRequest('/path');
    assert.equal(
      this.server.handledRequests.pop().url,
      `/path?region=${this.system.get('activeRegion')}`,
      'The region param is included when the system service shouldIncludeRegion property is true'
    );

    this.system.set('shouldIncludeRegion', false);

    token.authorizedRequest('/path');
    assert.equal(
      this.server.handledRequests.pop().url,
      '/path',
      'The region param is not included when the system service shouldIncludeRegion property is false'
    );
  });

  test('authorizedRequest does not include the region param when the region param is already in the URL', function (assert) {
    const token = this.subject();

    token.authorizedRequest('/path?query=param&region=already-here');
    assert.equal(
      this.server.handledRequests.pop().url,
      '/path?query=param&region=already-here',
      'The region param that is already in the URL takes precedence over the region in the service'
    );
  });

  test('authorizedRawRequest bypasses adding the region param', function (assert) {
    const token = this.subject();

    token.authorizedRawRequest('/path');
    assert.equal(
      this.server.handledRequests.pop().url,
      '/path',
      'The region param is ommitted when making a raw request'
    );
  });
});
