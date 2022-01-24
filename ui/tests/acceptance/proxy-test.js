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

    // Simulate a reverse proxy injecting X-Nomad-Token header for all requests
    this._originalXMLHttpRequestSend = XMLHttpRequest.prototype.send;
    (function (send) {
      XMLHttpRequest.prototype.send = function (data) {
        this.setRequestHeader('X-Nomad-Token', managementToken.secretId);
        send.call(this, data);
      };
    })(this._originalXMLHttpRequestSend);
  });

  hooks.afterEach(function () {
    XMLHttpRequest.prototype.send = this._originalXMLHttpRequestSend;
  });

  test('when token is inserted by a reverse proxy, the UI is adjusted', async function (assert) {
    // when token is inserted by reserve proxy, the token is reverse proxy
    const { secretId } = managementToken;

    await Jobs.visit();
    assert.equal(
      window.localStorage.nomadTokenSecret,
      null,
      'No token secret set'
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
