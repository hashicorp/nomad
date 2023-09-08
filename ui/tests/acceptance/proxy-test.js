/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable ember-a11y-testing/a11y-audit-called */ // Tests for non-UI behaviour.
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import Jobs from 'nomad-ui/tests/pages/jobs/list';

let managementToken;

module('Acceptance | reverse proxy', function (hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function () {
    window.localStorage.clear();
    window.sessionStorage.clear();

    server.create('agent');
    managementToken = server.create('token');

    // Prepare a setRequestHeader that accumulate headers already set. This is to avoid double setting X-Nomad-Token
    this._originalXMLHttpRequestSetRequestHeader =
      XMLHttpRequest.prototype.setRequestHeader;
    (function (setRequestHeader) {
      XMLHttpRequest.prototype.setRequestHeader = function (header, value) {
        if (!this.headers) {
          this.headers = {};
        }
        if (!this.headers[header]) {
          this.headers[header] = [];
        }
        // Add the value to the header
        this.headers[header].push(value);
        setRequestHeader.call(this, header, value);
      };
    })(this._originalXMLHttpRequestSetRequestHeader);

    // Simulate a reverse proxy injecting X-Nomad-Token header for all requests
    this._originalXMLHttpRequestSend = XMLHttpRequest.prototype.send;
    (function (send) {
      XMLHttpRequest.prototype.send = function (data) {
        if (!this.headers || !('X-Nomad-Token' in this.headers)) {
          this.setRequestHeader('X-Nomad-Token', managementToken.secretId);
        }
        send.call(this, data);
      };
    })(this._originalXMLHttpRequestSend);
  });

  hooks.afterEach(function () {
    XMLHttpRequest.prototype.setRequestHeader =
      this._originalXMLHttpRequestSetRequestHeader;
    XMLHttpRequest.prototype.send = this._originalXMLHttpRequestSend;
  });

  test('when token is inserted by a reverse proxy, the UI is adjusted', async function (assert) {
    // when token is inserted by reserve proxy, the token is reverse proxy
    const { secretId } = managementToken;

    await Jobs.visit();
    assert.equal(
      window.localStorage.nomadTokenSecret,
      secretId,
      'Token secret was set'
    );

    // Make sure that server received the header
    assert.ok(
      server.pretender.handledRequests
        .mapBy('requestHeaders')
        .every((headers) => headers['X-Nomad-Token'] === secretId),
      'The token header is always present'
    );

    assert.notOk(Jobs.runJobButton.isDisabled, 'Run job button is enabled');
  });
});
